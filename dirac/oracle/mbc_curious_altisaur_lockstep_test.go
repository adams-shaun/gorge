package oracle

import "testing"

// Curious Altisaur's combat-damage triggers and Invasion of Ixalan's defeat
// cast (CR 310.11) in one combat: gorge posed the trigger ordering and dirac
// posed the transformed-cast offer first (decision 898, pool tail-59).
func TestMBCCuriousAltisaurReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "curious_altisaur_trigger_order_before_defeat_cast")
}
