package oracle

import "testing"

// Rude Awakening's entwined cast beside Diamond Kaleidoscope's Prism tokens
// (pair 1 of tail-12 chunk c000, autopay, seed 787572, a Black-Green deck
// against a White-Black one): gorge offered the cast at decision 937 and the
// kernel's priority menu held only pass. The same cause as
// kicked_sac_repeat_gap (TestMBCRudeAwakeningReprosLockstep): the kernel
// prices the kicked total through the sacrifice-any repeat units. The
// recording diverged at main fd13a69db1 and replays clean since the
// kicked-sacrifice-repeat fix.
func TestMBCRudeAwakeningEntwineReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "entwined_cast_prism_tail_pair")
}
