package oracle

import "testing"

// A priority decision (kind ref priority) must offer every mode the kernel
// offers beside pass: at the recorded decision gorge offered only
// pass(-,-,0) while the kernel offered mode(-,-,0) and mode(-,-,1), so the
// engines parted at the option list of a priority pass (evloop
// dr-mbc-aaba, pair 15, seed 11690).
func TestMBCPriorityPassBesideKernelModeOptionsLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "priority_pass_beside_kernel_mode_options")
}
