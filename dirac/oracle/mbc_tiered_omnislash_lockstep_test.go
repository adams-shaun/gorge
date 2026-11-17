package oracle

import "testing"

// CR 702.183a, 601.2f: Cloud's Limit Break's Omnislash tier adds {3}{W} to
// its printed {1}{W}; at the mode ask a tier whose total cannot be paid is
// not offered. The recording stops at the modes decision (tail-26-c003).
func TestMBCCloudSLimitBreakReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tiered_omnislash_unaffordable_mode")
}
