package oracle

import "testing"

// CR 601.2h with CR 117.3: Viashino Outrider's echo cost {2}{R} is paid at
// p0's upkeep with Manabarbs on both sides. gorge taps Mountain#21, Mountain#16
// and Swamp#33; dirac taps Mountain#16, Swamp#29 and Swamp#31 — the projection
// first differs at Cards[20].Flags.
func TestMBCEchoCostLandChoiceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_echo_cost_land_choice")
}
