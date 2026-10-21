package view

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestLibraryOrderIsShapeOnlyForItsOwnerAndController: CR 401.2 hides a
// library's order from its owner, and CR 720.4's widening ("you may look at
// all cards that player could see") therefore cannot hand it to the seat
// controlling that owner either. Every Seat-visibility viewer is served the
// LibraryOrder's shape; a Secret look Note (the owner's own knowledge) still
// passes to the owner and the controller (guard: the rule-1 owner arm is
// narrowed for the library-order kinds only). cr-g61.
func TestLibraryOrderIsShapeOnlyForItsOwnerAndController(t *testing.T) {
	g := state.NewGame([]string{"a", "b", "c"})
	g.ControlledBy = map[state.PlayerID]state.PlayerID{1: 0}
	order := events.Event{Seq: 3, Kind: events.LibraryOrder, Player: 1, IDs: []state.ObjID{9, 8, 7}, Secret: true}
	look := events.Event{Seq: 4, Kind: events.Note, Player: 1, From: state.ZLibrary, Text: "looks at the top of the library", IDs: []state.ObjID{9}, Secret: true}
	for _, viewer := range []state.PlayerID{1, 0, 2} {
		got := RedactEventFor(g, order, viewer, Seat)
		if want := secretShape(order); got.Kind != want.Kind || got.Player != want.Player || !got.Secret || len(got.IDs) != 0 || got.Obj != 0 {
			t.Fatalf("viewer %d is served %+v for seat 1's LibraryOrder, want its shape only", viewer, got)
		}
		if blind := RedactEventForPregameBlind(g, order, viewer, Seat); len(blind.IDs) != 0 {
			t.Fatalf("viewer %d is served LibraryOrder IDs %v under the pregame blind", viewer, blind.IDs)
		}
	}
	for _, viewer := range []state.PlayerID{1, 0} {
		if got := RedactEventFor(g, look, viewer, Seat); len(got.IDs) != 1 || got.Text != look.Text {
			t.Fatalf("viewer %d lost seat 1's own look Note: %+v", viewer, got)
		}
	}
	if got := RedactEventFor(g, look, 2, Seat); len(got.IDs) != 0 {
		t.Fatalf("seat 2 is served seat 1's look Note IDs %v", got.IDs)
	}
}
