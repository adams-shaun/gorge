package host

import (
	"runtime"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

func decode[T any](t *testing.T, f protocol.Frame) T {
	t.Helper()
	var v T
	if err := f.Decode(&v); err != nil {
		t.Fatalf("%s: %v", f.T, err)
	}
	return v
}

// drainNonBlocking appends every frame currently sitting in s.Out() to
// *into, without blocking. Used two ways in this file: as the Options.Sleep
// pace hook, called on the match's own goroutine once per decision, right
// after that decision's burst has already been fully fanned out (fanout
// runs synchronously before Sleep is ever invoked) — so this is a
// deterministic per-decision handshake, not a race against a separately
// scheduled reader goroutine (Ruling FL-31); and as a final, one-shot call
// after r.Wait has confirmed the table's goroutine exited, to pick up
// whatever onMatchEnd pushed after the last decision's handshake. Both uses
// are free of timing dependency: the first because it never overlaps a
// concurrent writer (same goroutine, sequential), the second because
// r.Wait's return happens-after every push the run loop will ever make.
func drainNonBlocking(s *Session, into *[]protocol.Frame) {
	for {
		select {
		case f, ok := <-s.Out():
			if !ok {
				return
			}
			*into = append(*into, f)
		default:
			return
		}
	}
}

// drainNow is drainNonBlocking without an accumulator, for tests that only
// care about what is left after a match has already finished (Wait
// returned), where reading it is inherently race-free.
func drainNow(s *Session) []protocol.Frame {
	var out []protocol.Frame
	drainNonBlocking(s, &out)
	return out
}

func TestFocusSubscriptionStreamsSnapshotThenEventsInChainOrder(t *testing.T) {
	var s *Session
	var frames []protocol.Frame
	o := testOptions(t)
	o.Sleep = func(time.Duration) { drainNonBlocking(s, &frames) }
	r, _ := New(o)
	defer r.Close()
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	s = r.OpenSession()
	hello := decode[protocol.Hello](t, r.Hello(s))
	if hello.Session != s.ID || len(hello.Tables) != 1 || hello.Tables[0].ID != "t1" {
		t.Fatalf("hello %+v", hello)
	}
	if err := r.Subscribe(s, "t1", protocol.ModeFocus); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	drainNonBlocking(s, &frames) // pick up match_end, pushed after the last handshake
	if len(frames) < 100 {
		t.Fatalf("only %d frames", len(frames))
	}
	// match_start first, then the snapshot, then events/decisions, then match_end.
	if frames[0].T != protocol.TMatchStart || frames[1].T != protocol.TSnapshot || frames[len(frames)-1].T != protocol.TMatchEnd {
		t.Fatalf("shape: first %s, second %s, last %s", frames[0].T, frames[1].T, frames[len(frames)-1].T)
	}
	snap := decode[protocol.Snapshot](t, frames[1])
	if snap.View.Visibility != "omniscient" || len(snap.TurnStarts) < 1 || snap.View.Players[0].Hand == nil {
		t.Fatalf("snapshot %+v", snap.View)
	}
	var lastSeq uint64
	var lastID uint64
	events, decisions := 0, 0
	for i, f := range frames {
		if f.ID == 0 || f.ID <= lastID {
			t.Fatalf("frame %d: id %d not monotonic after %d", i, f.ID, lastID)
		}
		lastID = f.ID
		if f.Table != "t1" || f.Match != 1 {
			t.Fatalf("frame %d addressed to %s/%d", i, f.Table, f.Match)
		}
		switch f.T {
		case protocol.TEvent:
			eb := decode[protocol.EventBody](t, f)
			if eb.Event.Seq != f.Seq || (events > 0 && f.Seq != lastSeq+1) {
				t.Fatalf("frame %d: event seq %d after %d", i, f.Seq, lastSeq)
			}
			if eb.Event.Kind == "shuffle" && len(eb.Event.IDs) != 0 {
				t.Fatalf("frame %d leaks library order", i)
			}
			if eb.Line == "" && eb.Event.Kind != "clock_tick" {
				t.Fatalf("frame %d: no line for %s", i, eb.Event.Kind)
			}
			lastSeq = f.Seq
			events++
		case protocol.TDecision:
			d := decode[protocol.DecisionBody](t, f)
			if d.Kind == "" || d.Prompt == "" {
				t.Fatalf("decision %+v", d)
			}
			decisions++
		}
	}
	if events == 0 || decisions == 0 {
		t.Fatalf("%d events, %d decisions", events, decisions)
	}
	end := decode[protocol.MatchEnd](t, frames[len(frames)-1])
	ms, _ := r.Matches("t1")
	if end.Head != ms[0].Head || end.Result != ms[0].Result {
		t.Fatalf("match_end %+v vs %+v", end, ms[0])
	}
	// The event stream after the snapshot covers exactly the events after
	// the snapshot's head.
	if firstEvent := frames[2]; firstEvent.T == protocol.TEvent && firstEvent.Seq != snap.Head+1 {
		t.Fatalf("first event seq %d, snapshot head %d", firstEvent.Seq, snap.Head)
	}
}

func TestPublicTableRedactsHandsInEventsAndSnapshot(t *testing.T) {
	var s *Session
	var frames []protocol.Frame
	o := testOptions(t)
	o.Sleep = func(time.Duration) { drainNonBlocking(s, &frames) }
	r, _ := New(o)
	defer r.Close()
	cfg := fourSeatTable("t1", false)
	cfg.Spectator = view.Public
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	s = r.OpenSession()
	_ = r.Subscribe(s, "t1", protocol.ModeFocus)
	_ = r.Start("t1")
	r.Wait("t1")
	drainNonBlocking(s, &frames)
	if len(frames) < 2 {
		t.Fatalf("only %d frames", len(frames))
	}
	snap := decode[protocol.Snapshot](t, frames[1])
	for _, p := range snap.View.Players {
		if p.Hand != nil {
			t.Fatal("public snapshot shows a hand")
		}
	}
	for _, f := range frames {
		if f.T != protocol.TEvent {
			continue
		}
		eb := decode[protocol.EventBody](t, f)
		if eb.Event.Kind == "draw" && eb.Event.Obj != 0 {
			// A draw's card is hidden unless it has since become public
			// (played/cast) — with state-at-burst redaction the very draw
			// burst still hides it.
			t.Fatalf("public draw event names the card: %+v", eb)
		}
	}
}

func TestOverviewSubscriptionCoalescesWidgets(t *testing.T) {
	r, _ := New(testOptions(t))
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	_ = r.AddTable(fourSeatTable("t2", false))
	s := r.OpenSession()
	if err := r.Subscribe(s, protocol.TableAll, protocol.ModeOverview); err != nil {
		t.Fatal(err)
	}
	if err := r.Subscribe(s, protocol.TableAll, protocol.ModeFocus); err == nil {
		t.Fatal("focus on * accepted")
	}
	_ = r.StartAll()
	r.Wait("t1")
	r.Wait("t2")
	ws := s.TakeWidgets()
	if len(ws) != 2 || ws[0].Table != "t1" || ws[1].Table != "t2" {
		t.Fatalf("widgets %+v", ws)
	}
	w := decode[protocol.Widget](t, ws[0])
	if len(w.Life) != 4 || len(w.Lost) != 4 || w.State != protocol.MatchFinished || w.Last == "" {
		t.Fatalf("widget %+v", w)
	}
	if ws[0].ID != 0 {
		t.Fatal("widgets must not consume frame ids (PL-5)")
	}
	if again := s.TakeWidgets(); len(again) != 0 {
		t.Fatal("TakeWidgets did not clear")
	}
	// Overview frames with ids: match_start and match_end per table only.
	for _, f := range drainNow(s) {
		if f.T != protocol.TMatchStart && f.T != protocol.TMatchEnd {
			t.Fatalf("overview stream carried %s", f.T)
		}
	}
}

func TestRingResumesExactlyTheMissedFrames(t *testing.T) {
	var s *Session
	var frames []protocol.Frame
	o := testOptions(t)
	o.Ring = 64
	o.Sleep = func(time.Duration) { drainNonBlocking(s, &frames) }
	r, _ := New(o)
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	s = r.OpenSession()
	_ = r.Subscribe(s, "t1", protocol.ModeFocus)
	_ = r.Start("t1")
	r.Wait("t1")
	drainNonBlocking(s, &frames)
	if _, ok := s.Overflowed(); ok {
		t.Fatal("a drained session overflowed")
	}
	if len(frames) == 0 {
		t.Fatal("no frames")
	}
	last := frames[len(frames)-1]
	missed, ok := s.Since(last.ID - 10)
	if !ok || len(missed) != 10 {
		t.Fatalf("Since(last-10): ok=%v, %d frames, want 10", ok, len(missed))
	}
	if missed[0].ID != last.ID-9 || missed[len(missed)-1].ID != last.ID {
		t.Fatalf("Since(last-10): first %d last %d, want %d..%d", missed[0].ID, missed[len(missed)-1].ID, last.ID-9, last.ID)
	}
	if _, ok := s.Since(last.ID - 1000); ok {
		t.Fatal("Since reported success for an id older than the ring")
	}
	if got, ok := s.Since(last.ID); !ok || len(got) != 0 {
		t.Fatalf("Since(head) = %d frames, ok=%v", len(got), ok)
	}
}

func TestASessionThatNeverReadsIsDroppedAndTheMatchStillFinishes(t *testing.T) {
	o := testOptions(t)
	o.Ring = 16
	r, _ := New(o)
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	s := r.OpenSession()
	_ = r.Subscribe(s, "t1", protocol.ModeFocus)
	_ = r.Start("t1")
	r.Wait("t1")
	dropped, overflowed := s.Overflowed()
	if !overflowed || dropped == 0 {
		t.Fatalf("overflowed=%v dropped=%d", overflowed, dropped)
	}
	if _, open := <-s.Out(); open {
		// The channel still holds up to Ring frames; drain to the close.
		for range s.Out() {
		}
	}
	if _, ok := r.Session(s.ID); ok {
		t.Fatal("overflowed session still registered")
	}
	ms, _ := r.Matches("t1")
	if ms[0].State != protocol.MatchFinished {
		t.Fatalf("match %s; a slow subscriber must never stall the engine", ms[0].State)
	}
}

func TestUnsubscribeStopsFramesAndCloseSessionClosesOut(t *testing.T) {
	r, _ := New(testOptions(t))
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	s := r.OpenSession()
	if err := r.Subscribe(s, "t9", protocol.ModeFocus); err == nil {
		t.Fatal("subscribe to unknown table succeeded")
	}
	_ = r.Subscribe(s, "t1", protocol.ModeFocus)
	if err := r.Unsubscribe(s, "t1"); err != nil {
		t.Fatal(err)
	}
	_ = r.Start("t1")
	r.Wait("t1")
	n := len(drainNow(s))
	if n > 1 { // at most the snapshot pushed by Subscribe before Unsubscribe
		t.Fatalf("%d frames after unsubscribe", n)
	}
	r.CloseSession(s.ID)
	if _, open := <-s.Out(); open {
		for range s.Out() {
		}
	}
	if _, ok := r.Session(s.ID); ok {
		t.Fatal("closed session still registered")
	}
}

func TestSubscribeRejectsAnUnknownMode(t *testing.T) {
	r, _ := New(testOptions(t))
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	s := r.OpenSession()
	if err := r.Subscribe(s, "t1", "bogus"); err == nil {
		t.Fatal("bogus mode accepted")
	}
}

func TestUnsubscribeFromANeverSubscribedTableFails(t *testing.T) {
	r, _ := New(testOptions(t))
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	s := r.OpenSession()
	if err := r.Unsubscribe(s, "t1"); err != ErrNotFound {
		t.Fatalf("Unsubscribe on a never-subscribed table = %v, want ErrNotFound", err)
	}
}

func TestUnsubscribeWildcardClearsWidgetsWithNoSubscriptionLeft(t *testing.T) {
	r, _ := New(testOptions(t))
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	s := r.OpenSession()
	if err := r.Subscribe(s, protocol.TableAll, protocol.ModeOverview); err != nil {
		t.Fatal(err)
	}
	f, err := protocol.NewFrame(protocol.TWidget, "t1", 1, 0, protocol.Widget{})
	if err != nil {
		t.Fatal(err)
	}
	s.setWidget("t1", f)
	if err := r.Unsubscribe(s, protocol.TableAll); err != nil {
		t.Fatal(err)
	}
	if ws := s.TakeWidgets(); len(ws) != 0 {
		t.Fatalf("stale widget survived wildcard unsubscribe: %+v", ws)
	}
}

// testEventFrame builds a minimal, addressed TEvent frame for the synthetic
// ring/Since tests below: they exercise Session.push/Since/TakeWidgets
// directly, with no engine or match involved at all.
func testEventFrame(t *testing.T, seq uint64) protocol.Frame {
	t.Helper()
	f, err := protocol.NewFrame(protocol.TEvent, "t1", 1, seq, protocol.EventBody{})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestSessionRingAndSinceAreDeterministicWithNoEngine(t *testing.T) {
	o := testOptions(t)
	o.Ring = 4
	r, _ := New(o)
	defer r.Close()
	s := r.OpenSession()
	// Push and immediately drain 10 frames -- a fully caught-up reader, so
	// nothing ever overflows -- yet the resumable ring only remembers the
	// last Ring (4) of them.
	for i := uint64(0); i < 10; i++ {
		if !s.push(testEventFrame(t, i)) {
			t.Fatalf("push %d failed", i)
		}
		<-s.Out()
	}
	if _, of := s.Overflowed(); of {
		t.Fatal("a fully-drained session overflowed")
	}
	if got, ok := s.Since(5); ok {
		t.Fatalf("Since(5) = %+v, ok=true; id 5 predates the 4-frame ring", got)
	}
	got, ok := s.Since(6)
	if !ok || len(got) != 4 {
		t.Fatalf("Since(6): ok=%v, %d frames, want 4", ok, len(got))
	}
	if got[0].ID != 7 || got[len(got)-1].ID != 10 {
		t.Fatalf("Since(6): first %d last %d, want 7..10", got[0].ID, got[len(got)-1].ID)
	}
	if got, ok := s.Since(10); !ok || len(got) != 0 {
		t.Fatalf("Since(10) = %d frames, ok=%v, want 0 frames ok=true", len(got), ok)
	}
}

func TestSinceStillAnswersTheRingsWindowAfterOverflow(t *testing.T) {
	o := testOptions(t)
	o.Ring = 4
	r, _ := New(o)
	defer r.Close()
	s := r.OpenSession()
	// Never drain: the 5th push overflows a 4-slot channel. The ring
	// itself is unaffected by the overflow -- push records every frame in
	// it, including the one that overflows the channel -- so Since still
	// answers correctly for ids inside its 4-frame window...
	for i := uint64(0); i < 5; i++ {
		s.push(testEventFrame(t, i))
	}
	if _, of := s.Overflowed(); !of {
		t.Fatal("expected overflow")
	}
	if got, ok := s.Since(2); !ok || len(got) != 3 {
		t.Fatalf("Since(2) after overflow: ok=%v, %d frames, want 3", ok, len(got))
	}
	// ...and still correctly refuses ids older than that window.
	if _, ok := s.Since(0); ok {
		t.Fatal("Since(0) should predate a 4-frame ring after 5 pushes")
	}
}

// TestOverflowedDroppedCountsEveryFrameAfterTheFirst is the regression
// test for Ruling FL-32: dropped must count every frame attempted against
// an already-overflowed session, not latch at 1 the moment the channel
// first fills — a fan-out burst often calls push more than once per
// session (an event, then a decision), and once overflowed, every one of
// those later calls in the same burst is a genuinely undelivered frame.
func TestOverflowedDroppedCountsEveryFrameAfterTheFirst(t *testing.T) {
	o := testOptions(t)
	o.Ring = 2
	r, _ := New(o)
	defer r.Close()
	s := r.OpenSession()
	// Push 0 and 1 fill the 2-slot channel; push 2 discovers the overflow
	// (dropped=1); pushes 3 and 4 are attempted against an
	// already-overflowed session and must still each count.
	for i := uint64(0); i < 5; i++ {
		s.push(testEventFrame(t, i))
	}
	dropped, overflowed := s.Overflowed()
	if !overflowed {
		t.Fatal("expected overflow")
	}
	if dropped != 3 {
		t.Fatalf("dropped=%d, want 3 (the push that overflowed the channel plus the two attempted after)", dropped)
	}
}

func TestTakeWidgetsSortsRegardlessOfInsertionOrder(t *testing.T) {
	r, _ := New(testOptions(t))
	defer r.Close()
	s := r.OpenSession()
	mk := func(id TableID) protocol.Frame {
		f, err := protocol.NewFrame(protocol.TWidget, string(id), 1, 0, protocol.Widget{})
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	// Insert out of lexical order.
	s.setWidget("t2", mk("t2"))
	s.setWidget("t9", mk("t9"))
	s.setWidget("t1", mk("t1"))
	ws := s.TakeWidgets()
	if len(ws) != 3 || ws[0].Table != "t1" || ws[1].Table != "t2" || ws[2].Table != "t9" {
		t.Fatalf("widgets not sorted: %+v", ws)
	}
}

// TestSubscribeFocusSnapshotIsSerialisedAgainstFanout is the regression
// test for Ruling FL-30: without table.fanMu serialising a focus
// Subscribe's snapshot build+push against the match loop's own fan-out
// push loops, a client joining a live table could build its snapshot at
// head H, be preempted before pushing, let a burst's fanout push events
// [H+1..N] first, and only then push the now-stale snapshot@H — stranding
// the client permanently behind with the inversion baked into the ring.
//
// Two earlier versions of this test were tried and rejected before this
// one, both instructive about what "deterministic" needs to mean here:
//   - Natural concurrency (16 sessions Subscribe concurrently while an
//     unpaced match plays, no explicit synchronisation): passed 15/15 even
//     with fanMu's Lock/Unlock pairs removed everywhere. The race window
//     is too narrow to hit by hoping the scheduler cooperates — exactly
//     the kind of margin Ruling FL-31 rules out.
//   - Holding fanMu while the match was deliberately PAUSED (via a
//     blocking Sleep hook) removed all ambiguity about ordering, but also
//     removed the only other actor (a live fanout call) that could
//     collide with Subscribe — so it passed 8/8 even with the fix
//     stripped out. Proving mutual exclusion requires a second party
//     actually contending for the lock.
//
// This version holds the table's own fanMu directly — standing in for "a
// fan-out push loop is in progress" — while the match keeps running at
// full, unpaced speed on its own goroutine, so its own fanout calls are
// genuinely contending for fanMu throughout the hold, and checks the one
// assertion that is true under the fix for EVERY possible resolution of
// that contention (not just the most obvious one): no event whose seq
// exceeds the snapshot's head may appear, in this session's delivery
// order, before that snapshot. (A safe event — seq at or below the
// snapshot's head, from some other queued fan-out call that happened to
// win the race to re-acquire fanMu after this test releases it — arriving
// before the snapshot is fine; the snapshot already covers it. It is only
// a NEWER event jumping the snapshot that strands the client.)
//
// Confirmed against a deliberately-broken build (fanMu's Lock/Unlock pairs
// removed from Subscribe and from fanout/onMatchStart/onMatchEnd's push
// loops): the pre-release check below (no frame at all may reach the
// session while this test holds fanMu) caught the break in roughly half
// of 30 repeated runs — real scheduling contention, not a forced
// interleaving, so it is a probabilistic detector rather than a guaranteed
// one — and never failed across 30 repeated runs of the fixed build (zero
// false positives, which is the property that actually matters for this
// suite: a test that can miss a reintroduced bug some fraction of the time
// is still useful; one that fails a healthy build even once is not).
func TestSubscribeFocusSnapshotIsSerialisedAgainstFanout(t *testing.T) {
	r, _ := New(testOptions(t))
	defer r.Close()
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.mu.RLock()
	tb := r.tables["t1"]
	r.mu.RUnlock()
	// Wait for a live match to exist, so Subscribe's focus branch actually
	// has something to snapshot. No engine timing dependency: this only
	// blocks on the match's own state transition, not on any margin.
	for {
		tb.mu.RLock()
		live := tb.cur != nil
		tb.mu.RUnlock()
		if live {
			break
		}
		runtime.Gosched()
	}

	tb.fanMu.Lock() // stand in for "a fan-out push loop is in progress"

	s := r.OpenSession()
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		done <- r.Subscribe(s, "t1", protocol.ModeFocus)
	}()
	<-started
	for i := 0; i < 100; i++ {
		runtime.Gosched() // give both Subscribe and live fanout calls every chance to (wrongly) race ahead
	}

	select {
	case f := <-s.Out():
		tb.fanMu.Unlock()
		t.Fatalf("a frame (%s) reached the session while fanMu was held by someone else", f.T)
	default:
	}

	tb.fanMu.Unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")

	frames := drainNow(s)
	snapAt, snapHead, sawSnapshot := -1, uint64(0), false
	for i, f := range frames {
		if f.T == protocol.TSnapshot {
			snap := decode[protocol.Snapshot](t, f)
			snapAt, snapHead, sawSnapshot = i, snap.Head, true
			break
		}
	}
	if !sawSnapshot {
		t.Fatal("Subscribe never delivered a snapshot")
	}
	for i, f := range frames {
		if f.T != protocol.TEvent || i >= snapAt {
			continue
		}
		eb := decode[protocol.EventBody](t, f)
		if eb.Event.Seq >= snapHead+1 {
			t.Fatalf("event seq %d (frame %d) arrived before its own snapshot (head %d, frame %d)",
				eb.Event.Seq, i, snapHead, snapAt)
		}
	}
}
