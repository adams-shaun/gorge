package oracle

import "testing"

// Borrowed Malevolence (Escalate {2}, Charm 1-2): after the first mode is
// picked, gorge offered done/mode while the kernel went straight to targeting
// (decision 231 of tail-06 pair 5).
func TestMBCBorrowedMalevolenceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "escalate_second_mode_unaffordable")
}
