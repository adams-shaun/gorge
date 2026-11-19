package oracle

import "testing"

// The tail-76 pair-5 focus game (focus_00083_UR vs focus_00298_WR) asked a
// damage-replacement choice the kernel has no model for and stalled unmodelled
// (replplus) after 565 decisions. The recording is that stall.
func TestMBCTail76ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "replplus")
}
