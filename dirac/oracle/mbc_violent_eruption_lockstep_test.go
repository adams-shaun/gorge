package oracle

import "testing"

// Violent Eruption's madness trigger (CR 702.35a-b): Windfall discards the
// hand, the exiled Violent Eruption's owner is asked whether to cast it for
// its madness cost. gorge offered "yes" and the kernel only "no"
// (tail-16-c000 pair 11, decision 97).
func TestMBCViolentEruptionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "madness_cast_offer_divided_damage")
}
