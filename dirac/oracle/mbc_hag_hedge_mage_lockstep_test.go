package oracle

import "testing"

// Hag Hedge-Mage's priority cast offer at decision 564 of tail-55 pair 160
// (evidence loop, seed 105341): gorge offered the cast and the kernel, at land
// ec5795494d, did not. Later dirac mana-offer fixes made the kernel offer it
// too; the recording keeps both engines' priority menus equal.
func TestMBCHagHedgeMageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hag_hedge_mage_priority_cast_offer")
}
