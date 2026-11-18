package oracle

import "testing"

// Molder (X G instant) at a priority decision with {G} available: X may be 0,
// so the cast is affordable and must be offered beside pass. The kernel once
// offered only pass (decision 154 of 154; pool tail-22 pair 9).
func TestMBCMolderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_molder_x_zero_cast_offer")
}
