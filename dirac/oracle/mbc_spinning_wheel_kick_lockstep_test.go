package oracle

import "testing"

// CR 601.2b-c: X is announced before targets, and a spell whose chain link
// needs X targets (Spinning Wheel Kick: its damage link takes TargetMin$ X) can
// only be cast at an X those targets can be announced at. gorge's X ask judged
// the root's targets only, so it offered X up to what the mana could pay while
// the kernel stopped at the creatures and planeswalkers on the table
// (tail-44, decision 414: gorge 0..6, kernel 0..5).
func TestMBCSpinningWheelKickReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "x_announce_bounded_by_chain_targets")
}
