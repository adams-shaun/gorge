package oracle

import "testing"

// CR 107.3, CR 605.1a, CR 601.2g-h: p1 holds Pestilent Wolf's {2}{G} activation
// payable from Dwarven Hold's two storage counters and Joiner Adept's granted
// land mana beside it. Gorge prices the payment; the kernel's grant search
// dropped the counters of a battery that is itself a grant holder (decision 803
// of tail-08 pair 153).
func TestMBCStorageBatteryGrantHolderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "storage_battery_grant_holder_activation")
}
