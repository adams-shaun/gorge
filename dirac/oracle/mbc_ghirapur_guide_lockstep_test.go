package oracle

import "testing"

// Ghirapur Guide's ability and the lockstep choose decision it rides on: the
// two engines must offer the same options (tail-52-c002 pair 15, decision 278).
func TestMBCGhirapurGuideReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ghirapur_guide_choose_dig")
}
