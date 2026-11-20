package oracle

import "testing"

// CR 702.35a: when a card with madness is discarded it goes to exile and its
// madness trigger lets the owner cast it for the madness cost or put it into
// the graveyard. Alms of the Vein discarded by p1 (decision 123 of
// tail-34-c000 pair 4) was offered yes/no by dirac but only no by gorge.
func TestMBCAlmsOfTheVeinReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "alms_of_the_vein_madness_offer")
}
