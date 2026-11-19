package oracle

import "testing"

// CR 106.6, CR 605.3b: Chimeric Mass ({X}, an artifact spell) cast with a
// Powerstone, six basic lands and Viridescent Bog ({1},{T}: Add {B}{G}) offers
// X up to 8: the Powerstone's {C} pays the spell, and one basic pays the Bog's
// fee, so the filter nets one mana. dirac priced the restricted source and the
// filter apart and took the larger, so it capped the ask at 7
// (decision 299, tail-12-c000 pair 0).
func TestMBCChimericMassReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spell_x_bound_counts_powerstone_beside_filter_fee")
}
