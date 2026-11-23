package oracle

import "testing"

// Unholy Officiant's {4}{W} +1/+1 counter ability, with a copy of it already on
// the stack and lands still untapped: gorge offered another activation at
// priority and the kernel offered only pass (decision 481, tail-51-c000 pair 19).
func TestMBCUnholyOfficiantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_pain_repeats_fund_activation_offer")
}
