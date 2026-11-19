package oracle

import "testing"

// CR 702.148a: a prototype card may be cast by paying its prototype cost; the
// face cast is a normal cast of the object's prototype face (gorge's
// "prototype" face route), so both engines must offer it from the hand.
func TestMBCDepthChargeColossusReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "prototype_face_cast")
}
