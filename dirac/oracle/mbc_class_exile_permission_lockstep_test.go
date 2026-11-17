package oracle

import "testing"

// CR 406.6 and 607.2a: Rogue Class's level 3 lets its controller play the
// cards its level-1 trigger exiled face down. A Dig's face-down exile used to
// overwrite the exiled-with record the reading Class had just written, so the
// kernel offered none of those cards where gorge offered the land and the
// spell (Turbulent Wilderness and Impulse, decision 525 of 2790).
func TestMBCTurbulentWildernessReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "class_exile_permission_after_face_down_dig")
}
