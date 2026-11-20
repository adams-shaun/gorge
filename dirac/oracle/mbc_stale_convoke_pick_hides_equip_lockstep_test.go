package oracle

import "testing"

// CR 702.51a with CR 733.1: a Sundering Vitae cast that convoke tapped Sage of
// the Maze for, then aborted for want of a target, leaves its committed picks
// in the build scratch. The next priority decision must not read them: Sage of
// the Maze still taps for two mana, so Neurok Hoversail's Equip {2} is offered
// beside the cast. Gorge offered the activation and the kernel did not
// (decision 972 of tail-36 pair 64: its stale pick made the source look
// committed, so the equip priced unaffordable).
func TestMBCNeurokHoversailReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "stale_convoke_pick_hides_equip")
}
