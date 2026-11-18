package oracle

import "testing"

// Far // Away (a fuse split card) in hand: gorge offers castface(Far) at a
// priority decision where dirac offers only pass.
func TestMBCFarReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "far_fuse_castface_offer")
}
