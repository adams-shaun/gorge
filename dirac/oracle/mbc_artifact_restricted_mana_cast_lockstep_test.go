package oracle

import "testing"

// The kernel's filter-search mirror paid a filter's activation fee from the
// restricted batches the activation admits before ordinary mana, while
// gorge's payment spends the visible pool in colour order and carves a batch
// only by what its colour's spend actually consumed. Vedalken Engineer's
// artifact-only batch beside Stonework Packbeast's {2} fee: the kernel's
// spend left the Plains' {W} floating (the fee had eaten the batch), so the
// search priced a Donate cast beside the untapped Island payable that
// gorge's search — which kept the restricted {U} the fee could not spend —
// prices insufficient and offers [pass].
func TestMBCDonateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "artifact_restricted_mana_cast")
}
