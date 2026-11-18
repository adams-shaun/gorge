package oracle

import "testing"

// CR 605.1a, 601.2g: a land's {4},{T} activation is offered when the other
// lands plus a storage battery (Dwarven Hold, one storage counter) cover the
// {4}. Library's Investigate ability was offered by gorge and withheld by dirac
// (tail-08-c002 pair 103, decision 359).
func TestMBCLibraryReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "storage_battery_activation_generic_cost")
}
