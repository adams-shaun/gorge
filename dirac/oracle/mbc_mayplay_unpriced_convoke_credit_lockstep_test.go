package oracle

import "testing"

// CR 702.51a with 601.2b: a permission cast convokes too, so Rogue Class's
// level 3 lets its controller play an exiled Artistic Refusal with the taps
// of six creatures alone. The kernel's unpriced planner picture priced the
// raw {4}{U}{U} without that credit, refused the cast and offered only the
// pass and the land where gorge offered the may-play cast beside them
// (decision 642 of 642).
func TestMBCMayPlayUnpricedIgnoresConvokeCreditReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mayplay_unpriced_ignores_convoke_credit")
}
