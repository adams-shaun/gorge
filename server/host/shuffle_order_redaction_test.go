package host

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEventsSeatHidesLibraryShuffleOrder is the serving half of CR 701.20a:
// the owner's own EventsSeat stream must not carry a library Shuffle's
// ordered IDs, because a shuffled library's order is known to nobody. The
// transcript line and the event's shape still survive.
func TestEventsSeatHidesLibraryShuffleOrder(t *testing.T) {
	t.Parallel()
	r, id := startHumanTable(t)
	waitPending(t, r, id, 1, 0)

	_, m, err := r.lookup(id, 1)
	if err != nil {
		t.Fatal(err)
	}
	m.mu.RLock()
	rawShuffles := 0
	for _, ev := range m.e.L.Events {
		if ev.Kind == events.Shuffle && len(ev.IDs) > 0 {
			rawShuffles++
		}
	}
	m.mu.RUnlock()
	if rawShuffles == 0 {
		t.Fatal("fixture precondition: the log carries no library Shuffle with IDs")
	}

	for _, owner := range []state.PlayerID{0, 1} {
		bodies, err := r.EventsSeat(id, 1, 0, owner)
		if err != nil {
			t.Fatalf("EventsSeat(seat %d): %v", owner, err)
		}
		shuffles := 0
		for _, b := range bodies {
			if b.Event.Kind != "shuffle" {
				continue
			}
			shuffles++
			if len(b.Event.IDs) != 0 {
				t.Errorf("seat %d: served shuffle seq %d leaks its order: %v", owner, b.Event.Seq, b.Event.IDs)
			}
			if b.Event.Player != uint8(owner) {
				continue
			}
			if !strings.Contains(b.Line, "shuffles their library") {
				t.Errorf("seat %d: own shuffle seq %d lost its transcript line: %q", owner, b.Event.Seq, b.Line)
			}
		}
		if shuffles == 0 {
			t.Fatalf("seat %d: no shuffle events served", owner)
		}
	}
}
