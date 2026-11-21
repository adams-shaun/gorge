package oracle

import "testing"

// CR 702.26b with CR 701.54a: a phased-out permanent is treated as though it
// doesn't exist, so a Ring-bearer ask never offers it. Rainbow Efreet phased
// itself out and the controller's Uruk-hai Berserker then tempted them; gorge
// listed the phased-out Efreet among the eligible creatures while dirac did
// not (decision 295 of tail-50 pair 29).
func TestMBCRainbowEfreetReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ring_bearer_phased_out")
}
