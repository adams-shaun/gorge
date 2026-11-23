package oracle

import "testing"

// Chalice of the Void (X X) cast with Chief Engineer's convoke after tapping
// creatures, one of them Saprazzan Legate: gorge and dirac offered different
// X choices (decision 431 of tail-14-c000 pair 63).
func TestMBCSaprazzanLegateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "saprazzan_legate_chalice_convoke_x")
}
