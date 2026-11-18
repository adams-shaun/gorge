package oracle

import "testing"

// Lunar Frenzy's X choice: gorge and dirac must offer the same X values, and
// pay the announced X through the cast window's life-only source (Treasonous
// Ogre) the same way.
func TestMBCLunarFrenzyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lunar_frenzy_x_offer")
	stdReproNamed(t, "lunar_frenzy_x_life_pay")
}
