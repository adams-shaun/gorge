package oracle

import "testing"

// CR 106.6, 118.12: Energy Flux gives every artifact "at the beginning of your
// upkeep, sacrifice this artifact unless you pay {2}". With one Island and two
// Powerstone tokens left untapped (the Powerstone's mana can't be spent to cast
// a nonartifact spell, but an unless cost is no spell), gorge offers both the
// pay and the decline choices; dirac once offered only the decline
// (dr-mbc-aaab pair 128, decision 438 of 438).
func TestMBCBrokenWingsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_cost_paid_with_powerstone_mana")
}

// CR 106.6: a Powerstone's {C} "can't be spent to cast a nonartifact spell",
// so it pays an unless cost, which is no spell's payment. Reduce counters its
// caster's own Skyshaper unless {3}; the caster holds an untapped Vivid
// Meadow, Island and Powerstone, so gorge offers the pay and dirac offered the
// decline alone (modes decision 371 of 371, tail-56 pair 438).
func TestMBCReduceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_cost_paid_with_powerstone")
}
