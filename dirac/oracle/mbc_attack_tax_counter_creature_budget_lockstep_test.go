package oracle

import "testing"

// CR 508.1d, 508.1g: Nils, Discipline Enforcer taxes every creature with
// counters {X} (its counters) to attack, and the controller's untapped
// Mistvault Bridge pays {1}. High Sentinels of Arashin carries one counter, so
// it attacks for {1}; the kernel offered it while gorge did not (decision 543,
// tail-63 pair 49).
func TestMBCHighSentinelsOfArashinReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "attack_tax_counter_creature_budget")
}
