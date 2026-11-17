package oracle

import "testing"

// CR 702.142a: a Boast ability may be activated only if its creature attacked
// this turn. Tuskeri Firewalker blocked on the opponent's turn and never
// attacked, so gorge offers only pass at the post-combat priority; the kernel
// also offered the Boast activation (decision 371, tail-39-c005 pair 3).
func TestMBCTuskeriFirewalkerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "boast_requires_attacked_this_turn")
}
