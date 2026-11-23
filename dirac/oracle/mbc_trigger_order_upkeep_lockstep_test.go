package oracle

import "testing"

// Two upkeep triggers of one controller (Oath of Druids' and Thassa, God of the
// Sea's) put on the stack in the order the controller chose: gorge and dirac
// parted on which one sits at Stack[0] after the order answer (decision 618 of
// pool tail-06, pair 13).
func TestMBCThassaGodOfTheSeaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_order_upkeep_gods_and_oath")
}
