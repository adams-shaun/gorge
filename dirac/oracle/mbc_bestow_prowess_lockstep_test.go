package oracle

import "testing"

// CR 702.103b with 702.108: a bestowed Triton Wavebreaker is cast as an Aura
// spell, a noncreature spell, so each prowess instance on the controller's
// creatures triggers. The target Wavebreaker has prowess twice (its own as a
// creature, and the grant from the bestowed one already attached to it), so
// gorge asks to order two triggers where dirac, treating the Aura spell as a
// creature spell, queued none (decision 391; tail-22-c000 pair 5).
func TestMBCTritonWavebreakerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bestow_spell_is_noncreature_for_prowess")
}
