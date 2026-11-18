package oracle

import "testing"

// Halo Forager's "you may pay {X}" announcement: the X values offered are the
// ones the controller's sources can pay. gorge offered 0..2 where the kernel
// offered 0..3 (decision 636 of 636; tail-41-c000 pair 31, seed 22846).
func TestMBCHaloForagerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "halo_forager_pay_x")
}
