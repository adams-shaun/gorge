package oracle

import "testing"

// Screeching Bat's upkeep trigger ("you may pay {2}{B}{B}") with one untapped
// Swamp and a Dwarven Hold holding one storage counter: the two sources reach
// two of the four mana, so the election is an over-bound offer that fails at
// the charge. The engines parted while paying (decision 564; tail-08 pair 5).
func TestMBCScreechingBatReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_cost_storage_battery")
}
