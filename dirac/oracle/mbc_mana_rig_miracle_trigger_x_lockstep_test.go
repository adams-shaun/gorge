package oracle

import "testing"

// A Temporal Mastery drawn by a Clue while The Mana Rig's X = 1 ability is
// still on the stack: the miracle trigger that goes above it is no
// {X} object, so its stack X is 0 in both engines (tail-01 pair 612).
func TestMBCManaRigMiracleTriggerStackXLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mana_rig_miracle_trigger_stack_x")
}
