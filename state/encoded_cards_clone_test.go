package state

import "testing"

// TestCloneDeepDoesNotAliasEncodedCards pins the EncodedCards carrier Cipher
// adds (state.Object.EncodedCards, written by effects/cipher.go's api:Cipher
// and folded by the Imprint kind's "encoded" discriminator). CloneDeep must
// independently back it like every other association slice: a long-lived clone
// (rules.Engine.Clone/snapshotTriggerBoard's LKI capture, Game.Clone) that
// shared the live object's backing array could see the live object's later
// Imprint append, or a live append with spare capacity could write into the
// clone's array.
func TestCloneDeepDoesNotAliasEncodedCards(t *testing.T) {
	orig := Object{ID: 7, EncodedCards: []ObjID{8, 9}}
	clone := orig.CloneDeep()
	if len(clone.EncodedCards) != 2 || clone.EncodedCards[0] != 8 || clone.EncodedCards[1] != 9 {
		t.Fatalf("clone EncodedCards = %v, want [8 9]", clone.EncodedCards)
	}

	clone.EncodedCards[0] = 80
	if orig.EncodedCards[0] == 80 {
		t.Fatal("CloneDeep aliases EncodedCards")
	}
	if orig.EncodedCards[0] != 8 {
		t.Fatalf("original EncodedCards[0] = %d, want 8", orig.EncodedCards[0])
	}

	// A live append with spare capacity must not reach into the clone either.
	orig.EncodedCards = append(orig.EncodedCards, 10)
	if len(clone.EncodedCards) != 2 {
		t.Fatalf("appending to the original grew the clone's EncodedCards to %v", clone.EncodedCards)
	}
}
