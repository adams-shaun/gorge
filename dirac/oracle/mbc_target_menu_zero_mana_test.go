package oracle

import "testing"

// CR 601.2c/601.2h with CR 605.1: Fantastic Bounce costs {2} less only if it
// targets a tapped creature, and its caster holds {U}{W} with Cabal Coffers
// as the only other source. Coffers makes {B} for each Swamp and there is none,
// so it adds nothing and only the tapped creatures are payable targets. Gorge
// treated the zero-Swamp Amount$ as an unpriced production, took the window
// census as incomplete and offered every nonland permanent
// (decision 845 of tail-41 pair 137, evidence loop).
func TestMBCArachnoformReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "target_menu_prices_zero_amount_mana_source")
}
