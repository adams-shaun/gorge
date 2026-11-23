package oracle

import "testing"

// CR 616.1: two Lashknife Barriers both rewrite the combat damage dealt to
// their controller's creature, and gorge asks the affected player which
// applies next (decision 818 of tail-55 pair 0, Ethereal Investigator blocked
// by two creatures); the kernel dealt the damage without the ask. The kernel
// stalls loud (StallReplPlus) on two minus lines meeting one event, as it does
// for the other competitions it does not pose, so the recording is a declared
// stall rather than a divergence.
func TestMBCEtherealInvestigatorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "damage_minus_pair_order_ask")
}
