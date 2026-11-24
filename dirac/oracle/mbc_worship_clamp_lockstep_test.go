package oracle

import "testing"

// CR 616.1: two Worships on the battlefield both clamp the same combat damage
// to leave the player at 1 life. The clamps give one event in either order, so
// the affected player has no real choice to make; gorge once posed a
// replacement-order decision between them that dirac, applying them in turn,
// never offered (decision 767 of tail-55-c000, Harbin attacking into two
// Worships).
func TestMBCHarbinVanguardAviatorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "worship_loss_clamps_commute")
}
