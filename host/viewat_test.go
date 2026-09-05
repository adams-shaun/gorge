package host

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/replay"
	"github.com/adams-shaun/gorge/view"
)

func finishedTable(t *testing.T) (*Registry, *match) {
	t.Helper()
	r, _ := New(testOptions(t))
	t.Cleanup(func() { r.Close() })
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	_ = r.Start("t1")
	r.Wait("t1")
	r.mu.RLock()
	tb := r.tables["t1"]
	r.mu.RUnlock()
	return r, tb.history[0]
}

func viewJSON(t *testing.T, v view.View) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestBoundsOfMatchesTheLoopsOwnBookkeeping(t *testing.T) {
	_, m := finishedTable(t)
	got := boundsOf(m.e.L.Events)
	if len(got) != len(m.bounds) {
		t.Fatalf("boundsOf found %d boundaries, the loop recorded %d", len(got), len(m.bounds))
	}
	for i := range got {
		if got[i] != m.bounds[i] {
			t.Fatalf("boundary %d: derived %d, recorded %d", i, got[i], m.bounds[i])
		}
	}
	// One snapshot at genesis plus one per burst that began a turn; a burst
	// can contain two turn changes only in degenerate games, so at most one
	// snapshot per turn start.
	if len(m.snaps) < 3 || len(m.snaps) > len(m.turnStarts) {
		t.Fatalf("%d snapshots for %d turn starts", len(m.snaps), len(m.turnStarts))
	}

	// FL-43 (fix round 1): a partial final burst — the shape a crashed
	// match's log has, since Submit records its intent before
	// handle/checkStateBased/Advance can panic on it — must not be
	// mistaken for a complete boundary. boundsOf's last element must stay
	// at the previous real boundary, not grow to the truncated length.
	l, k := truncatedMidBurst(t, m)
	partial := boundsOf(l.Events)
	if len(partial) != k+1 {
		t.Fatalf("partial burst: boundsOf found %d boundaries, want %d (one per completed intent, boundary 0 = genesis)", len(partial), k+1)
	}
	if partial[len(partial)-1] != m.bounds[k] {
		t.Fatalf("partial burst: boundsOf's last boundary is %d, want the previous real one %d — the truncated tail must not count",
			partial[len(partial)-1], m.bounds[k])
	}
}

// truncatedMidBurst returns a copy of m's log cut strictly inside some
// burst — after at least one of its events, before the DecisionAsk or
// GameOver that would complete it — plus k, the number of intents that
// fully completed before the truncated (k+1-th) one. This is the shape a
// crashed match's own log has (spec D15, FL-43): Submit appends the
// intent to L.Intents before handle/checkStateBased/Advance run, so a
// panic partway through leaves a straggler tail with no ask and no
// GameOver, and L.Intents one entry longer than any complete boundary
// accounts for.
func truncatedMidBurst(t *testing.T, m *match) (*events.Log, int) {
	t.Helper()
	for k := 0; k+1 < len(m.bounds); k++ {
		if m.bounds[k+1] > m.bounds[k]+1 {
			cut := m.bounds[k] + 1
			return &events.Log{
				Seed:    m.e.L.Seed,
				Events:  append([]events.Event(nil), m.e.L.Events[:cut]...),
				Intents: append([]decision.Intent(nil), m.e.L.Intents[:k+1]...),
			}, k
		}
	}
	t.Skip("fixture has no multi-event burst to truncate")
	return nil, 0
}

func TestViewAtFromSnapshotsEqualsViewAtFromGenesis(t *testing.T) {
	_, m := finishedTable(t)
	head := uint64(len(m.e.L.Events) - 1)
	rng := rand.New(rand.NewPCG(1, 2))
	seqs := []uint64{0, m.bounds[0] - 1, m.bounds[0], head}
	// Each seq costs a full replay from genesis on the slow path, so the
	// sample is kept small: 6 random seqs plus every third turn start and
	// its successor. 40 random + every turn start ran ~60s here and past
	// the 10-minute package timeout under -race (Ruling FL-49).
	for i := 0; i < 6; i++ {
		seqs = append(seqs, uint64(rng.IntN(int(head)+1)))
	}
	for i := 0; i < len(m.turnStarts); i += 3 {
		ts := m.turnStarts[i]
		seqs = append(seqs, ts, ts+1)
	}
	for _, seq := range seqs {
		if seq > head {
			continue
		}
		fast, err := viewAt(m.cfg, m.e.L, m.snaps, seq, view.Omniscient)
		if err != nil {
			t.Fatalf("seq %d (snapshots): %v", seq, err)
		}
		slow, err := viewAt(m.cfg, m.e.L, nil, seq, view.Omniscient)
		if err != nil {
			t.Fatalf("seq %d (genesis): %v", seq, err)
		}
		if a, b := viewJSON(t, fast), viewJSON(t, slow); a != b {
			t.Fatalf("seq %d: snapshot path differs from full replay\n%s\n%s", seq, a, b)
		}
	}

	// Independent oracle: at a handful of boundary-aligned seqs, viewAt
	// must agree with an entirely separate reconstruction — replay's own
	// package, not host's snapshot/genesis machinery at all — of the
	// engine after exactly that many intents. This is the check that
	// would catch a bug shared by both of viewAt's own paths above (e.g.
	// an off-by-one in boundsOf or the j/from selection), which comparing
	// them only against each other cannot.
	step := len(m.bounds) / 6
	if step < 1 {
		step = 1
	}
	for j := 0; j < len(m.bounds); j += step {
		seq := m.bounds[j] - 1
		got, err := viewAt(m.cfg, m.e.L, m.snaps, seq, view.Omniscient)
		if err != nil {
			t.Fatalf("oracle seq %d (j=%d): %v", seq, j, err)
		}
		oracle, err := replay.ReplayTo(m.e.L, m.cfg, j)
		if err != nil {
			t.Fatalf("oracle replay to boundary %d: %v", j, err)
		}
		want := view.ProjectFor(oracle.G, oracle, view.NoSeat, view.Omniscient, nil)
		if a, b := viewJSON(t, got), viewJSON(t, want); a != b {
			t.Fatalf("seq %d (j=%d): viewAt disagrees with replay.ReplayTo\n%s\n%s", seq, j, a, b)
		}
	}
}

func TestViewAtHeadEqualsTheLiveProjection(t *testing.T) {
	r, m := finishedTable(t)
	head := uint64(len(m.e.L.Events) - 1)
	got, err := r.ViewAt("t1", 1, head)
	if err != nil {
		t.Fatal(err)
	}
	want := view.ProjectFor(m.e.G, m.e, view.NoSeat, view.Omniscient, nil)
	if viewJSON(t, got) != viewJSON(t, want) {
		t.Fatal("ViewAt(head) differs from projecting the live engine")
	}
	if got.Over != true {
		t.Fatal("finished match's head view is not Over")
	}
}

func TestViewAtTracksIntraBurstStateChanges(t *testing.T) {
	_, m := finishedTable(t)
	// Find a LifeChange or Damage-to-player event and check the life total
	// moves at exactly that seq — inside a burst, not at its boundary.
	for _, ev := range m.e.L.Events {
		if ev.Kind != events.LifeChange || ev.Amount == 0 {
			continue
		}
		before, err := viewAt(m.cfg, m.e.L, m.snaps, ev.Seq-1, view.Omniscient)
		if err != nil {
			t.Fatal(err)
		}
		after, err := viewAt(m.cfg, m.e.L, m.snaps, ev.Seq, view.Omniscient)
		if err != nil {
			t.Fatal(err)
		}
		if after.Players[ev.Player].Life-before.Players[ev.Player].Life != ev.Amount {
			t.Fatalf("seq %d: life moved %d, event says %d", ev.Seq,
				after.Players[ev.Player].Life-before.Players[ev.Player].Life, ev.Amount)
		}
		return
	}
	t.Skip("fixture produced no life change")
}

func TestViewAtAndEventsErrors(t *testing.T) {
	r, m := finishedTable(t)
	head := uint64(len(m.e.L.Events) - 1)
	_, err := r.ViewAt("t1", 1, head+1)
	var beyond ErrBeyondHead
	if !errors.As(err, &beyond) || beyond.Head != head {
		t.Fatalf("beyond head: %v", err)
	}
	if _, err := r.ViewAt("t9", 1, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown table: %v", err)
	}
	if _, err := r.ViewAt("t1", 2, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown match: %v", err)
	}
	if _, err := r.Events("t1", 1, head+1); !errors.As(err, &beyond) {
		t.Fatalf("events beyond head: %v", err)
	}
}

// TestViewAtSurvivesATruncatedFinalBurst is FL-43's regression test: a
// log shaped like a crashed match's (a straggler tail from the last,
// unfinished burst — see truncatedMidBurst) must not make viewAt(head)
// resubmit the unfinished intent, whether or not it panics for real.
// Fixed, this resolves to a clean view built from the previous boundary
// plus events.Apply of the tail — via a real snapshot AND via a from-
// genesis replay.ReplayTo, which must agree with each other too.
func TestViewAtSurvivesATruncatedFinalBurst(t *testing.T) {
	_, m := finishedTable(t)
	l, k := truncatedMidBurst(t, m)
	head := uint64(len(l.Events) - 1)

	fromSnaps, err := viewAt(m.cfg, l, m.snaps, head, view.Omniscient)
	if err != nil {
		t.Fatalf("viewAt(head) with snapshots panicked or errored: %v", err)
	}
	fromGenesis, err := viewAt(m.cfg, l, nil, head, view.Omniscient)
	if err != nil {
		t.Fatalf("viewAt(head) from genesis panicked or errored: %v", err)
	}
	if a, b := viewJSON(t, fromSnaps), viewJSON(t, fromGenesis); a != b {
		t.Fatalf("truncated burst: snapshot path differs from genesis replay\n%s\n%s", a, b)
	}
	if fromSnaps.Over {
		t.Fatal("a truncated mid-burst log is never a finished game")
	}

	// The unfinished intent (l.Intents[k]) must never have been resubmitted:
	// confirm by reconstructing the same state the hard way, from the
	// previous complete boundary plus a direct events.Apply of the one
	// straggler event — no Submit involved anywhere past boundary k.
	base, err := replay.ReplayTo(l, m.cfg, k)
	if err != nil {
		t.Fatalf("replay to the last complete boundary: %v", err)
	}
	if uint64(len(base.L.Events)) != m.bounds[k] {
		t.Fatalf("replay to boundary %d produced %d events, want %d", k, len(base.L.Events), m.bounds[k])
	}
	for s := m.bounds[k]; s <= head; s++ {
		events.Apply(base.G, l.Events[s])
	}
	want := view.ProjectFor(base.G, base, view.NoSeat, view.Omniscient, nil)
	if a, b := viewJSON(t, fromSnaps), viewJSON(t, want); a != b {
		t.Fatalf("truncated burst: viewAt differs from the manual apply-only reconstruction\n%s\n%s", a, b)
	}
}

// TestViewAtAndEventsDuringALiveMatch exercises ViewAt/Events from a
// second goroutine while the match loop is still writing to the same
// match — the concurrency -race is meant to catch. finishedTable (every
// other test here) only reads after Wait, so on its own -race never sees
// a reader overlap a writer.
func TestViewAtAndEventsDuringALiveMatch(t *testing.T) {
	r, _ := New(testOptions(t))
	t.Cleanup(func() { r.Close() })
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var seq uint64
		for {
			select {
			case <-stop:
				return
			default:
			}
			switch _, err := r.ViewAt("t1", 1, seq); {
			case err == nil:
				seq++
			case errors.As(err, new(ErrBeyondHead)), errors.Is(err, ErrNotFound):
				// Not ready yet, or we ran past a head that keeps moving.
			default:
				t.Errorf("ViewAt(%d): %v", seq, err)
				return
			}
			if _, err := r.Events("t1", 1, 0); err != nil &&
				!errors.As(err, new(ErrBeyondHead)) && !errors.Is(err, ErrNotFound) {
				t.Errorf("Events: %v", err)
				return
			}
		}
	}()
	r.Wait("t1")
	close(stop)
	wg.Wait()
}

// TestViewAtAndEventsRedactForAPublicTable is the redaction negative for
// the Public (no-seat) spectator visibility, mirroring
// TestEventsSinceReturnsRedactedDescribedTail's Omniscient/shuffle check:
// a Public ViewAt never carries a hand, and a Public Events never carries
// the Obj of a hidden-to-hidden move (view.RedactEvents rule 2).
func TestViewAtAndEventsRedactForAPublicTable(t *testing.T) {
	r, _ := New(testOptions(t))
	t.Cleanup(func() { r.Close() })
	cfg := fourSeatTable("t1", false)
	cfg.Spectator = view.Public
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	r.mu.RLock()
	m := r.tables["t1"].history[0]
	r.mu.RUnlock()
	head := uint64(len(m.e.L.Events) - 1)

	v, err := r.ViewAt("t1", 1, head)
	if err != nil {
		t.Fatal(err)
	}
	if v.Visibility != view.Public.String() {
		t.Fatalf("visibility %q, want %q", v.Visibility, view.Public.String())
	}
	for _, p := range v.Players {
		if p.Hand != nil {
			t.Fatalf("public ViewAt exposes seat %d's hand: %v", p.ID, p.Hand)
		}
	}

	bodies, err := r.Events("t1", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	strippedAny := false
	for _, b := range bodies {
		if b.Event.Kind == "draw" && b.Event.Obj == 0 {
			strippedAny = true
		}
	}
	if !strippedAny {
		t.Fatal("public Events never stripped a draw's Obj — fixture has no hidden-to-hidden move to check")
	}
}

func TestEventsSinceReturnsRedactedDescribedTail(t *testing.T) {
	r, m := finishedTable(t)
	head := uint64(len(m.e.L.Events) - 1)
	all, err := r.Events("t1", 1, 0)
	if err != nil || len(all) != int(head)+1 || all[0].Event.Seq != 0 || all[len(all)-1].Event.Seq != head {
		t.Fatalf("Events(0): %d bodies, %v", len(all), err)
	}
	tail, _ := r.Events("t1", 1, head-4)
	if len(tail) != 5 || tail[0].Event.Seq != head-4 {
		t.Fatalf("Events(head-4): %d bodies from %d", len(tail), tail[0].Event.Seq)
	}
	for _, b := range all {
		if b.Event.Kind == "shuffle" && len(b.Event.IDs) != 0 {
			t.Fatal("Events leaks library order")
		}
		if b.Line == "" && b.Event.Kind != "clock_tick" {
			t.Fatalf("seq %d (%s) has no line", b.Event.Seq, b.Event.Kind)
		}
	}
	_ = protocol.TEvent
}
