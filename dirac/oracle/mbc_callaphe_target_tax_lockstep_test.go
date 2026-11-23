package oracle

import "testing"

// CR 601.2c, 601.2f: Callaphe, Beloved of the Sea taxes the spells an
// opponent casts that target her side's creatures and enchantments. The
// target menu of the opponent's Field of Reality must agree between the engines
// (decision 524 of tail-20-c000).
func TestMBCCallapheBelovedOfTheSeaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "callaphe_target_tax_menu")
}
