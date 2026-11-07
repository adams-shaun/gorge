package events

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// CR 733.1-2: a spell cast from the middle of a library and then reversed
// goes back to its own place in that library, not to the bottom (where a
// plain move into a library appends it).
func TestReversedLibraryCastKeepsItsPlace(t *testing.T) {
	g, l := twoPlayer(t)
	before := slices.Clone(g.Zone(state.ZLibrary, 0))
	if len(before) < 5 {
		t.Fatalf("library has %d cards, want at least 5", len(before))
	}
	id := before[3]
	Emit(g, l, Event{Kind: PutOnStack, Obj: id, From: state.ZLibrary, To: state.ZStack})
	if slices.Contains(g.Zone(state.ZLibrary, 0), id) {
		t.Fatalf("the cast card is still in the library after the push")
	}
	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZStack, To: state.ZLibrary, Text: "reversed"})
	if got := g.Zone(state.ZLibrary, 0); !slices.Equal(got, before) {
		t.Fatalf("library after the reversal = %v, want the pre-cast order %v", got, before)
	}
	if o := g.Obj(id); o.PreStackLibSlot != 0 {
		t.Fatalf("PreStackLibSlot = %d after the reversal, want it cleared", o.PreStackLibSlot)
	}
	// A plain stack->library move (not a reversal) still goes to the bottom.
	Emit(g, l, Event{Kind: PutOnStack, Obj: id, From: state.ZLibrary, To: state.ZStack})
	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZStack, To: state.ZLibrary})
	if lib := g.Zone(state.ZLibrary, 0); lib[len(lib)-1] != id {
		t.Fatalf("a non-reversal move into the library: %v, want %d at the bottom", lib, id)
	}
}
