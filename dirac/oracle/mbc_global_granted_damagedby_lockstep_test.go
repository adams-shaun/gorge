package oracle

import "testing"

// CR 603.10 / CR 613.1f: Vampiric Sliver grants every Sliver "whenever a
// creature dealt damage by this creature this turn dies, ..." so Battle
// Sliver and Vampiric Sliver both dealt combat damage to a dying Zombie Musher
// and both triggers go to the stack, in an order the controller chooses.
// dirac once recorded "damaged by" only for the sources whose own card reads
// it, so Battle Sliver's granted trigger never fired and the trigger_order
// decision offered only Vampiric Sliver's (decision 413, tail-08-c000 pair 3).
func TestMBCBattleSliverReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "global_granted_damagedby_trigger_second_sliver")
}
