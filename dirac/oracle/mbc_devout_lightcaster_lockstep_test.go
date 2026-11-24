package oracle

import "testing"

// CR 616.1: two Lashknife Barriers both rewrite the combat damage a blocker
// they protect is dealt, and gorge asks the affected player which applies next
// (decision 1058 of tail-55 pair 0 with the Lashknife Barrier / Worship focus:
// Fireborn Knight's damage to Devout Lightcaster). The two minus lines commute,
// so the kernel poses the same no-op ask instead of stalling loud.
func TestMBCDevoutLightcasterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "combat_damage_minus_pair_blocker_order_ask")
}
