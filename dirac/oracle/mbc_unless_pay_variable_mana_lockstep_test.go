package oracle

import "testing"

// CR 118.12 with CR 605.3a: a player facing "sacrifice unless you pay {2}"
// may activate mana abilities to pay it. Molting Harpy's upkeep trigger found
// its controller with one untapped Swamp and a Mercadian Bazaar carrying
// three storage counters ("{T}, remove any number of storage counters: add {R}
// for each"), so {2} was reachable and the pay option had to be offered. One
// engine offered only the decline (tail-26-c001, decision 665).
func TestMBCTowerOfCalamitiesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_pay_variable_storage_mana")
}
