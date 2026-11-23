package oracle

import "testing"

// Protect // Serve (a fuse split card): gorge and dirac must offer the same
// castable halves at p0's priority (decision 133 of pool tail-74-c003).
func TestMBCProtectReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "protect_fuse_half_offer")
}
