package host

import (
	"testing"

	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

// drain reads every frame until Out closes or the table finishes, returning
// them in order. Sessions are drained on the test goroutine while the
// table plays on its own, the same shape as the SSE writer.
func drain(t *testing.T, r *Registry, s *Session, id TableID) []protocol.Frame {
	t.Helper()
	var out []protocol.Frame
	done := make(chan struct{})
	go func() { r.Wait(id); close(done) }()
	for {
		select {
		case f, ok := <-s.Out():
			if !ok {
				return out
			}
			out = append(out, f)
		case <-done:
			for {
				select {
				case f, ok := <-s.Out():
					if !ok {
						return out
					}
					out = append(out, f)
				default:
					return out
				}
			}
		}
	}
}

func decode[T any](t *testing.T, f protocol.Frame) T {
	t.Helper()
	var v T
	if err := f.Decode(&v); err != nil {
		t.Fatalf("%s: %v", f.T, err)
	}
	return v
}

func TestFocusSubscriptionStreamsSnapshotThenEventsInChainOrder(t *testing.T) {
	r, _ := New(testOptions(t))
	defer r.Close()
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	s := r.OpenSession()
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
	frames := drain(t, r, s, "t1")
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
	r, _ := New(testOptions(t))
	defer r.Close()
	cfg := fourSeatTable("t1", false)
	cfg.Spectator = view.Public
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	s := r.OpenSession()
	_ = r.Subscribe(s, "t1", protocol.ModeFocus)
	_ = r.Start("t1")
	frames := drain(t, r, s, "t1")
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

// drainNow returns whatever is already buffered without blocking.
func drainNow(s *Session) []protocol.Frame {
	var out []protocol.Frame
	for {
		select {
		case f, ok := <-s.Out():
			if !ok {
				return out
			}
			out = append(out, f)
		default:
			return out
		}
	}
}

func TestRingResumesExactlyTheMissedFrames(t *testing.T) {
	o := testOptions(t)
	o.Ring = 64
	r, _ := New(o)
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	s := r.OpenSession()
	_ = r.Subscribe(s, "t1", protocol.ModeFocus)
	_ = r.Start("t1")
	frames := drain(t, r, s, "t1")
	if _, ok := s.Overflowed(); ok {
		t.Fatal("a drained session overflowed")
	}
	last := frames[len(frames)-1]
	missed, ok := s.Since(last.ID - 10)
	if !ok || len(missed) != 10 || missed[0].ID != last.ID-9 || missed[9].ID != last.ID {
		t.Fatalf("Since: ok=%v, %d frames, first %d last %d", ok, len(missed), missed[0].ID, missed[len(missed)-1].ID)
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
