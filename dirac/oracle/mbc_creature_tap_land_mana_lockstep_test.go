package oracle

import "testing"

// CR 601.2g / 605.3b: Garruk, Primal Hunter ({2}{G}{G}{G}) is castable from
// two Forests, Holdout Settlement (tapping a creature for any color), a
// Mountain and Rakdos Carnarium. Gorge omitted the cast from the priority menu
// that dirac offered (decision 235 of tail-42 pair 1).
func TestMBCGarrukPrimalHunterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "creature_tap_land_mana_garruk_cast")
}
