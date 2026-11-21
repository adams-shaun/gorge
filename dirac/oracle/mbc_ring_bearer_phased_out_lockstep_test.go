package oracle

import "testing"

// CR 702.26b: a phased-out permanent is treated as though it doesn't exist, so
// it isn't a creature the Ring can choose as Ring-bearer (CR 701.54a). Uruk-hai
// Berserker entering while Rainbow Efreet is phased out offered the Efreet
// among the bearer choices on the gorge side only (decision 304 of 304, pair 15
// of tail-50-s000).
func TestMBCRainbowEfreetReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ring_bearer_phased_out")
}
