//go:build dirac_mp

package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestFourSeatLivingPlayersBoundLockstep plays four-seat DFT pods of
// Riverchurn Monument, whose "any number of target players" slot is bounded
// by the living players (PlayerCountPlayers$Amount): the controller's
// living opponents and the controller, read as the slot is announced. No
// divergence and no stall; with the opponents counted as one
// (dirac.DebugOppCountOne) the slot closes early and some seed diverges.
func TestFourSeatLivingPlayersBoundLockstep(t *testing.T) {
	blue := deckOf(16, "Riverchurn Monument", deckOf(24, "Island")...)
	play := fourSeatAtkPlay(t, "std-dft", [4][]string{blue, blue, blue, blue})
	const seeds = 8
	fourSeatAtkClean(t, play, seeds)
	if !fourSeatAtkMutated(t, play, seeds, &dirac.DebugOppCountOne) {
		t.Fatal("counting the opponents as one changed no seed")
	}
}
