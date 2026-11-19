package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Two copies of a hand mana source (Elvish Spirit Guide's "Exile this card
// from your hand: Add {G}") are no twins for the prefix search: the kernel's
// twin cut (filtSearch.twinTried) reads battlefield slots alone and its hand
// pass (handFund) tries each guide in turn, so a gorge twin cut over the hand
// skips the second guide where the kernel walks it. The two diverge when the
// first guide is the searched play's own card: exiling it ends the play, and
// the second is the only guide that can fund it.
func TestScriptTwinIgnoresTwoHandSources(t *testing.T) {
	t.Parallel()
	e := boundBoard(t, []string{"Mountain"}, []string{"Elvish Spirit Guide", "Elvish Spirit Guide"})
	hand := e.G.Zone(state.ZHand, 0)
	var first state.ObjID
	for _, id := range hand {
		if scriptOptionKey(e.Pending(), id) != "" {
			first = id
		}
	}
	if first == 0 {
		t.Fatal("precondition: a guide in hand offers its mana ability")
	}
	// findAndMoveToHand returns the card already in hand for a repeated name:
	// bring the second copy of the same face over from the library by hand.
	var second state.ObjID
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() == e.G.Obj(first).Face() {
			second = id
			break
		}
	}
	if second == 0 {
		t.Fatal("precondition: a second guide in the library")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: second, From: state.ZLibrary, To: state.ZHand})
	e.pending = nil
	e.Advance()
	d := e.Pending()
	if d == nil {
		t.Fatal("precondition: a pending decision")
	}
	k0, k1 := scriptOptionKey(d, first), scriptOptionKey(d, second)
	if k0 == "" || k0 != k1 {
		t.Fatalf("precondition: both guides offer the same activate option, got %q and %q", k0, k1)
	}
	if scriptTwin(e.G, d, first, second) {
		t.Fatal("two guides in hand are no twins: the first may be the play's own card")
	}
}
