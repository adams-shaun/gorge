package oracle

import "testing"

// CR 601.2c, 601.2g: a spell's targets are chosen before its costs are paid, so
// Combo Attack (two target creatures you control, then a target creature) is
// castable with only a Forest, two Eldrazi Spawn tokens and a Plant: the Spawns
// may be both targeted and sacrificed for the {C} to pay the {2}. The kernel
// offered the cast at decision 415 of dr-mbc-bbab pair 19 and gorge did not.
func TestMBCComboAttackReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "two_target_cast_paid_with_its_own_targets")
}
