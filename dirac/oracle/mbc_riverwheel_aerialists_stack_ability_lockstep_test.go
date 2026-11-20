package oracle

import "testing"

// CR 702.108a: Riverwheel Aerialists's prowess is a triggered ability. The
// recorded game attacks with it and the trigger's stack entry must project the
// same ability identity on both engines.
func TestMBCRiverwheelAerialistsReprosLockstep(t *testing.T) {
	stdReproNamed(t, "riverwheel_aerialists_stack_ability")
}
