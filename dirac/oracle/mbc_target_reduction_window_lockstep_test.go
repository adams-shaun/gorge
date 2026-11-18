package oracle

import "testing"

// CR 601.2c, 601.2g: Fantastic Bounce costs {2} less if it targets a tapped creature. Its caster floats
// {U}{U} beside an untapped Xander's Lounge and River of Tears, whose Condition-split mana the payment
// planner defers, so the plain sources price only three of the unreduced {3}{U}. gorge's target menu
// then asks the complete CR 601.2g window census, which reaches the full cost, and keeps the untapped
// Mister Fantastic beside the tapped Alabaster Host Sanctifier; dirac once kept only the tapped creature
// (tail-41 pair 56, decision 254 of 254).
func TestMBCMisterFantasticReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "target_reduction_window_full_cost")
}
