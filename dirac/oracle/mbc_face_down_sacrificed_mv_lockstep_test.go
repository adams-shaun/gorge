package oracle

import "testing"

// CR 708.2 with CR 608.2h: a manifested card sacrificed to Birthing Ritual was
// a mana-value-0 face-down creature as it left the battlefield, so the dig's X
// (1 plus the sacrificed creature's mana value) is 1. The kernel read the
// printed card in the graveyard (a six-drop), offered the mana-value-6 creature
// among the top seven and left the trigger waiting, where gorge found nothing
// to take and put the whole window on the bottom (tail-12-c000 pair 2,
// decision 348).
func TestMBCTrompTheDomainsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "face_down_sacrificed_remembered_mv")
}
