package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestZoneGenAdvancesOnEveryZoneChange pins state.Object.ZoneGen (CR 400.7):
// every real zone change advances it, whichever zones are involved; a CR
// 733.1 reversed cast restores the value the card had before the proposal;
// and a move whose destination is the zone the object is already in is not
// a zone change.
func TestZoneGenAdvancesOnEveryZoneChange(t *testing.T) {
	g, l := twoPlayer(t)
	id := g.Zone(state.ZLibrary, 0)[0]
	gen := func() uint32 { return g.Obj(id).ZoneGen }
	start := gen()
	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZHand, To: state.ZGraveyard})
	if got := gen(); got != start+2 {
		t.Fatalf("after library->hand->graveyard ZoneGen = %d, want %d", got, start+2)
	}
	if inc := g.Obj(id).Incarnation; inc != 0 {
		t.Fatalf("Incarnation = %d, want 0: no battlefield crossing", inc)
	}
	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZHand})
	inHand := gen()
	Emit(g, l, Event{Kind: PutOnStack, Obj: id, From: state.ZHand, To: state.ZStack})
	if gen() != inHand+1 {
		t.Fatalf("the cast's move onto the stack: ZoneGen = %d, want %d", gen(), inHand+1)
	}
	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZStack, To: state.ZHand, Text: "reversed"})
	if gen() != inHand {
		t.Fatalf("reversed cast: ZoneGen = %d, want the pre-proposal %d", gen(), inHand)
	}
	Emit(g, l, Event{Kind: PutOnStack, Obj: id, From: state.ZHand, To: state.ZStack})
	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZStack, To: state.ZHand})
	if gen() != inHand+2 {
		t.Fatalf("a real stack->hand return (not a reversal): ZoneGen = %d, want %d", gen(), inHand+2)
	}
}
