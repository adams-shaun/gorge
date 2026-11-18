package oracle

import "testing"

// CR 509.1b with CR 702.111b: a menace attacker blocked by two creatures
// already meets its floor, so a third untapped creature may still join the
// block. With a lure (Nemesis Mask) elsewhere in combat the kernel's
// lureFloorMet counted only the creatures still free to block, not the ones
// already declared, and never offered the third blocker gorge offered
// (dr-mbc-baab pair 63, decision 464 of 464).
func TestMBCDreamscapeArtistReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lure_menace_floor_counts_declared_blockers")
}
