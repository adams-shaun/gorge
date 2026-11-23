package oracle

import "testing"

// CR 603.3d with CR 707.2: the copied-modal-trigger target ask again, found at
// decision 818 of the Battlefly Swarm game (dr-tail-49-c000 pair 121, seed
// 497848): gorge went on to priority and offered only pass where the kernel
// asked for the chosen mode's player target, because gorge resolved the copied
// trigger's mode bodies against the source's active face. Fixed with
// copied_trigger_mode_targets (6f917510f7); this records the second game of
// the class so a regression names Battlefly Swarm's pair.
func TestMBCBattleflySwarmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "copied_trigger_mode_targets_battlefly_pass")
}
