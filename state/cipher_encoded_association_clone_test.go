package state

import (
	"testing"
)

// bearLike is game_test.go's cardsFixture bear: any Creature permanent works.

// TestCipherEncodedAssociationGameClone pins Game.Clone's deep copy of the
// Cipher association (state.Object.EncodedCards) at the state layer, where the
// events package cannot be imported: the association fields are set here as
// test-fixture construction mirroring exactly what the Imprint "encoded" fold
// writes, and the assertions are about the CLONE, not about engine flow (the
// through-events clearing lives in events/cipher_encoded_clone_test.go).
func TestCipherEncodedAssociationGameClone(t *testing.T) {
	g := NewGame([]string{"a", "b"})
	bear, err := cardsFixture()
	if err != nil {
		t.Fatalf("fixture card: %v", err)
	}
	creature := g.AddObject(bear, 0)
	card := g.AddObject(bear, 0)
	// Re-read through g.Obj: AddObject may have grown g.Objs, so the pointers
	// AddObject returned can point into a stale backing array.
	creature = g.Obj(creature.ID)
	card = g.Obj(card.ID)
	card.Zone = ZExile
	g.SetZone(ZExile, 0, []ObjID{card.ID})
	creature.Zone = ZBattlefield
	g.SetZone(ZBattlefield, 0, []ObjID{creature.ID})
	// Fixture construction (the Imprint "encoded" fold's write shape): the
	// battlefield creature encodes the exiled card.
	creature.EncodedCards = append(creature.EncodedCards, card.ID)

	clone := g.Clone()
	if got := clone.Obj(creature.ID).EncodedCards; len(got) != 1 || got[0] != card.ID {
		t.Fatalf("precondition: clone EncodedCards = %v, want [%d]", got, card.ID)
	}
	// Aliasing probe: an in-place write through the clone's link must not
	// reach the live game (a shared backing array would move both).
	clone.Obj(creature.ID).EncodedCards[0] = 0xFFFF
	if got := g.Obj(creature.ID).EncodedCards; got[0] != card.ID {
		t.Fatalf("clone write reached the live game's EncodedCards: %v", got)
	}
	clone.Obj(creature.ID).EncodedCards[0] = card.ID

	// The clone owns its backing: a live append with spare capacity must not
	// grow the clone's list.
	g.Obj(creature.ID).EncodedCards = append(g.Obj(creature.ID).EncodedCards, 999)
	if got := clone.Obj(creature.ID).EncodedCards; len(got) != 1 {
		t.Fatalf("appending to the live game grew the clone's EncodedCards to %v", got)
	}

	// Clearing the clone's list leaves the live one untouched.
	clone.Obj(creature.ID).EncodedCards = nil
	if got := g.Obj(creature.ID).EncodedCards; len(got) != 2 || got[0] != card.ID || got[1] != 999 {
		t.Fatalf("clearing the clone's EncodedCards reached the live game: %v", got)
	}
}
