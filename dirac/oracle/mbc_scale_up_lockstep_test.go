package oracle

import "testing"

// CR 601.2c and CR 702.96a: Scale Up's normal mode targets a creature you
// control, and its overload mode doesn't target at all. At decision 442 of
// dr-mbc-abba pair 133 gorge offered cast(#34{Scale Up}) and the kernel didn't.
func TestMBCScaleUpReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "scale_up_offer")
}
