package oracle

import "testing"

// Wall of Bone was offered as a cast by dirac at decision 523 of tail-55 pair 60
// (seed 542963) while gorge offered only pass.
func TestMBCWallOfBoneReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "wall_of_bone_cast_offer")
}
