package oracle

import "testing"

// Riptide Survivor's face-down cast ({3}, CR 702.37b) was offered by dirac
// at a priority decision where gorge offered only the printed cast
// (decision 853, tail-09 pair 72).
func TestMBCRiptideSurvivorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "morph_cast_face_down_pool_pricing")
}
