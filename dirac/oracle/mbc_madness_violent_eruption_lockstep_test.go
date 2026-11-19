package oracle

import "testing"

// CR 702.35a: Violent Eruption is discarded into exile and its madness
// trigger asks whether to cast it for {1}{R}{R}. Both engines must pose the
// same yes/no offer (tail-16-c001 pair 76, seed 122806, decision 155).
func TestMBCViolentEruptionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "madness_violent_eruption")
}
