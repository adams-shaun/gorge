package oracle

import "testing"

// Kitchen Imp's madness ask (CR 702.35a-b) after a discard: both engines pose
// the cast-or-graveyard choice, and "yes" is offered on both or neither
// (tail-53 pair 658 at land 304323bb0f, decision 129).
func TestMBCKitchenImpReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "madness_cast_offer")
}
