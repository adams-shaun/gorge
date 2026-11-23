package oracle

import "testing"

// CR 509.1b / CR 613.11 class: Fifty Feet of Rope's "Target Wall can't block
// this turn" is a restriction effect, not an ability the Wall gains, so Sudden
// Spoiling's later "loses all abilities" can't end it. gorge keeps it (a hidden
// rule-effect keyword survives a removal); the kernel set the until-end-of-turn
// CantBlock grant aside with the Wall's other grants and offered Glacial Wall
// as a blocker of Scurrilous Sentry (decision 408, tail-06 pair 5).
func TestMBCGlacialWallReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cant_block_effect_outlives_loss_of_abilities")
}
