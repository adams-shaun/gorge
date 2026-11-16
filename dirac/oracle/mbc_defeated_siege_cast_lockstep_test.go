package oracle

import "testing"

// CR 310.11: a battle with no defense counters is exiled by the state-based
// action and its owner may cast it transformed. Gorge posed the
// defeat_cast_yes/no choice for Invasion of Kaladesh at decision 849 of pair
// 1262 (pool tail-10) where the kernel stayed at priority.
func TestMBCIvoraInsatiableHeirReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "defeated_siege_cast_choice")
}
