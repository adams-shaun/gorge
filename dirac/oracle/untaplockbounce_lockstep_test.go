package oracle

import "testing"

// A source of the untap lock bounced and recast is a new object (CR 400.7,
// 611.2b): the lock it made ended when it left, and the recast one locks only
// what its own enters trigger taps. The deck is the untap-lock one with half
// of the Wind Drakes swapped for Unsummon, so a Somnophore or Tidebinder Mage
// is returned to hand and cast again under gorge and the kernel alike.
var untapLockBounceNames = [2][]string{
	append(append(append(deckOf(4, "Somnophore"), deckOf(4, "Tidebinder Mage")...),
		append(deckOf(4, "Wind Drake"), deckOf(4, "Unsummon")...)...), deckOf(24, "Island")...),
	untapLockHostNames[1],
}

// TestUntapLockHostBounceLockstep: no seed diverges when the lock's source is
// bounced and recast, and in some a locked creature stayed tapped through its
// controller's untap step (so the lockstep runs real locks).
func TestUntapLockHostBounceLockstep(t *testing.T) {
	div, kept := untapLockHostRunDecks(t, untapLockBounceNames, 100)
	if div != 0 {
		t.Fatalf("%d seeds diverged", div)
	}
	if kept == 0 {
		t.Fatal("no locked creature stayed tapped through an untap step; the lockstep proves nothing")
	}
	t.Logf("untap steps that left a locked creature tapped: %d", kept)
}
