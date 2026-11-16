package oracle

import "testing"

// Everflowing Chalice (multikicker {2}) cast with the mana for a kick: the two
// engines must agree on whether the multikicker count is asked
// (dr-mbc-abab pair 21, decision 487 of 487).
func TestMBCEverflowingChaliceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "multikicker_chalice_choice")
}
