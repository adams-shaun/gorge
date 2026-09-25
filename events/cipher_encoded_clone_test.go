package events

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestCipherEncodedAssociationClone pins clone safety of the Cipher
// association through the event folds: Game.Clone (state.Object.CloneDeep per
// object) must give the clone its own EncodedCards backing, so clearing the
// association in one branch -- through events, never a direct write -- leaves
// the other branch's association exactly as it was.
func TestCipherEncodedAssociationClone(t *testing.T) {
	g, l, creature, card := cipherEncodeFixture(t)
	if got := g.Obj(creature).EncodedCards; len(got) != 0 {
		t.Fatalf("precondition: fresh creature already encodes %v", got)
	}
	for _, e := range cipherEncodeEvents(card, creature) {
		Emit(g, l, e)
	}
	assertCipherAssociation(t, g, creature, card, []state.ObjID{card})

	clone := g.Clone()
	// Precondition: the clone carries the same association before anything
	// diverges.
	if got := clone.Obj(creature).EncodedCards; len(got) != 1 || got[0] != card {
		t.Fatalf("precondition: clone EncodedCards = %v, want [%d]", got, card)
	}
	// Aliasing probe: an in-place write through the clone's link must not
	// reach the live game (a shared backing array would move both).
	clone.Obj(creature).EncodedCards[0] = state.ObjID(0xFFFF)
	if got := g.Obj(creature).EncodedCards; len(got) != 1 || got[0] != card {
		t.Fatalf("clone write reached the live game's EncodedCards: %v", got)
	}
	clone.Obj(creature).EncodedCards[0] = card

	// Live branch: the encoded card leaves exile through an event; the live
	// link is pruned and the clone's must survive untouched.
	Emit(g, l, Event{Kind: MoveZone, Obj: card, From: state.ZExile, To: state.ZHand})
	if got := g.Obj(creature).EncodedCards; len(got) != 0 {
		t.Fatalf("live branch kept the association after the card left exile: %v", got)
	}
	if got := clone.Obj(creature).EncodedCards; len(got) != 1 || got[0] != card {
		t.Fatalf("clearing the live branch reached into the clone: %v", got)
	}
	if ko := clone.Obj(card); ko == nil {
		t.Fatal("clone's encoded card object vanished")
	} else if ko.Zone != state.ZExile {
		t.Fatalf("clone's encoded card zone = %v, want exile", ko.Zone)
	}

	// Clone branch: the clone's OWN encoder leaves the battlefield through an
	// event on the clone; its link clears without touching the live game.
	Emit(clone, NewLog(1), Event{Kind: MoveZone, Obj: creature, From: state.ZBattlefield, To: state.ZGraveyard})
	if got := clone.Obj(creature).EncodedCards; len(got) != 0 {
		t.Fatalf("clone branch kept the association after its encoder left: %v", got)
	}
	if ko := clone.Obj(card); ko == nil {
		t.Fatal("clone's encoded card object vanished")
	} else if ko.Zone != state.ZExile {
		t.Fatalf("clone's encoded card moved with its encoder: zone = %v", ko.Zone)
	}
	// The live game is untouched by the clone's events: its encoder is still
	// a battlefield creature (it never left in this branch) with an empty
	// list from lifecycle A, and its card is still in hand.
	if co := g.Obj(creature); co.Zone != state.ZBattlefield || len(co.EncodedCards) != 0 {
		t.Fatalf("clone events reached the live game: zone %v, encoded %v",
			co.Zone, co.EncodedCards)
	}
	if ko := g.Obj(card); ko.Zone != state.ZHand {
		t.Fatalf("clone events moved the live card: zone = %v, want hand", ko.Zone)
	}
}
