package oracle

import "testing"

// CR 605.1a, 602.2b: an activation's {1} is paid by a Simian Spirit Guide
// exiled from hand, while the two lands its cost sacrifices are tapped. The
// board holds a Golgari Signet beside the guide, so dirac's census prices the
// filter and left the Keldon Arsonist activation out of the priority menu
// that gorge offered (decision 1092, tail-45-c000 pair 1).
func TestMBCKeldonArsonistReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spirit_guide_funds_sac_lands_activation")
}
