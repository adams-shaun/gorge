package oracle

import "testing"

// Subterfuge's Encore {7}{U}{U} (CR 702.141a) from the graveyard: gorge offered
// activate(Subterfuge) at a priority decision where the kernel did not
// (decision 625 of tail-76 pair 48).
func TestMBCSubterfugeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "encore_offer_from_graveyard")
}
