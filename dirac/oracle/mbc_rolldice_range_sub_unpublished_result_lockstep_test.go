package oracle

import "testing"

// Boing! ("Return target creature to its owner's hand, then roll a six-sided
// die. If the result is 3 or less, scry a number of cards equal to the
// result.") rolled 2: gorge's ResultSubAbilities$ range subs run before
// ResultSVar$ is published, so the sub's ScryNum$ Result read 0 and gorge
// scried nothing, while the kernel lowered the range sub inline after the die
// and waited for its scry arrange with the spell still on the stack (decision
// 73; tail-36 pair 7, the game the evidence loop filed under Boing!).
func TestMBCBoingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "rolldice_range_sub_unpublished_result")
}
