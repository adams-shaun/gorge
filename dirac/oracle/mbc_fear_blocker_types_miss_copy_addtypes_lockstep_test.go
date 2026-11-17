package oracle

import "testing"

// CR 702.36b with CR 707.9b/c: Auton Soldier entered as a copy of a Human
// creature with "except ... is an artifact in addition to its other types",
// so the copy IS an artifact creature and may block an attacking creature
// with fear. dirac read the blocker's printed card types (the copied
// creature's, no artifact) and refused the block; gorge offers it.
func TestMBCAutonSoldierReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "fear_blocker_types_miss_copy_addtypes")
}
