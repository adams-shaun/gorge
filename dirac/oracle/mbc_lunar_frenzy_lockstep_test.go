package oracle

import "testing"

// Lunar Frenzy's X choice: gorge and dirac must offer the same X values.
func TestMBCLunarFrenzyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lunar_frenzy_x_offer")
}
