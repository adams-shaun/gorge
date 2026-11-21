package oracle

import "testing"

// CR 702.26b with CR 701.54a: a phased-out creature is treated as though it
// doesn't exist, so it is not among the creatures the Ring tempts its
// controller to choose from. Rainbow Efreet had phased itself out when
// Uruk-hai Berserker's trigger tempted its controller; gorge still offered it
// as a Ring-bearer (decision 395 of tail-50-s000 pair 62).
func TestMBCRainbowEfreetReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ring_bearer_phased_out_creature")
}
