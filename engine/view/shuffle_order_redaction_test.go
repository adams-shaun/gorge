package view

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestLibraryShuffleOrderHiddenFromOwner is CR 701.20a's redaction leaf: a
// library Shuffle carries the randomized order in IDs, and NO seat may learn
// it from the event stream -- not an opponent, not the library's owner, and
// not the player controlling that owner (CR 720.4). The event's shape
// survives so the transcript line still renders.
func TestLibraryShuffleOrderHiddenFromOwner(t *testing.T) {
	g := fourSeatBoard(t)
	order := g.Zone(state.ZLibrary, 1)
	if len(order) < 2 {
		t.Fatalf("fixture precondition: seat 1's library has %d cards, want at least 2", len(order))
	}
	g.ControlledBy = map[state.PlayerID]state.PlayerID{1: 0}
	ev := events.Event{Seq: 9, Kind: events.Shuffle, Player: 1,
		From: state.ZLibrary, To: state.ZLibrary, Step: state.StepUpkeep,
		IDs: append([]state.ObjID(nil), order...), Secret: true}

	for _, tc := range []struct {
		name   string
		viewer state.PlayerID
	}{
		{"owner", 1},
		{"opponent", 2},
		{"controller", 0},
		{"spectator", NoSeat},
	} {
		got := RedactEvent(g, ev, tc.viewer)
		if len(got.IDs) != 0 {
			t.Errorf("%s: shuffle order leaked: %v", tc.name, got.IDs)
		}
		if got.Seq != ev.Seq || got.Kind != events.Shuffle || got.Player != 1 ||
			got.From != state.ZLibrary || got.To != state.ZLibrary ||
			got.Step != state.StepUpkeep || !got.Secret {
			t.Errorf("%s: shuffle event lost its shape: %+v", tc.name, got)
		}
	}

	// Positive control: the fixture's own IDs really are the order the event
	// carried, so the emptiness above is redaction and not an empty event.
	if len(ev.IDs) != len(order) {
		t.Fatalf("fixture precondition: event IDs %d, library %d", len(ev.IDs), len(order))
	}
}
