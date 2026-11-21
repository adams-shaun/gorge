package oracle

import "testing"

// Jace's Projection ({2}{U}{U}) was offered as a cast by gorge at priority
// but not by dirac (decision 562 of tail-49-c000, seed 467318).
func TestMBCJaceSProjectionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "jaces_projection_cast_offer")
}
