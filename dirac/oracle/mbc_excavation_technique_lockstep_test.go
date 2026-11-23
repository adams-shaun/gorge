package oracle

import "testing"

// Excavation Technique ({3}{W}) in tail-12-c000 pair 2: gorge offered the cast to p1
// and the kernel did not (decision 227).
func TestMBCExcavationTechniqueReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "excavation_technique_cast_offer")
}
