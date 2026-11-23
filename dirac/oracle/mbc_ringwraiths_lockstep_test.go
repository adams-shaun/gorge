package oracle

import "testing"

// Ringwraiths and Rot-Tide Gargantua block Spike Worker and kill it with
// combat damage (decision 624 of tail-36-c000 pair 45): the two engines
// disagree on the blockers' Blocks link once the attacker has died.
func TestMBCRingwraithsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ringwraiths_blockers_after_attacker_dies")
}
