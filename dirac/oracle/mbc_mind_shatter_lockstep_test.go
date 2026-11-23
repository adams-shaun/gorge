package oracle

import "testing"

// Mind Shatter ({X}{B}{B}) is offered at priority by gorge but not by dirac
// (decision 166, dr tail-16-c000 pair 127).
func TestMBCMindShatterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mind_shatter_offered_at_priority")
}
