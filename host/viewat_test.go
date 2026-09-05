package host

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
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
}

func TestViewAtFromSnapshotsEqualsViewAtFromGenesis(t *testing.T) {
	_, m := finishedTable(t)
	head := uint64(len(m.e.L.Events) - 1)
	rng := rand.New(rand.NewPCG(1, 2))
	seqs := []uint64{0, m.bounds[0] - 1, m.bounds[0], head}
	for i := 0; i < 40; i++ {
		seqs = append(seqs, uint64(rng.IntN(int(head)+1)))
	}
	for _, ts := range m.turnStarts {
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
