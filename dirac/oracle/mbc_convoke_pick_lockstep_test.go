package oracle

import "testing"

// CR 702.51a with CR 733.1: a Sundering Vitae cast that convoke tapped
// Sage of the Maze for, then aborted for want of a target, leaves its
// committed picks in the build scratch. The next priority decision must not
// read them: Sage of the Maze still taps for two mana, so Haunted Cloak's
// Equip {1} is offered (decision 1096 of tail-36-c000, pair 2; the kernel
// had dropped Equip because pickCommittedSrc read the stale pick).
func TestMBCHauntedCloakReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "stale_convoke_pick_hides_combo_any_source")
}
