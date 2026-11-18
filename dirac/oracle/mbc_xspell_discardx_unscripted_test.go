package oracle

import (
	"testing"
)

// Abandon Hope ({X}{1}{B} whose Cost$ adds an announced Discard<X/Card/card>)
// at X = 0 costs {1}{B} and discards nothing, and p0's Plains and Tendo Ice
// Bridge pay it -- yet gorge never offers the cast: its potential planner
// declines the additional-cost shape ("shape:additional_cost") and prices the
// composition only on its census, where Tendo Ice Bridge's any-colour ability
// (a SubCounter cost) is deferred, so an insufficient composition drops the
// play and no prefix search runs for it (potential_plan.go
// potentialPlayVerdictShared). The kernel scripted the census gap and offered
// the cast beside gorge's menu without it (decision 73 of 73; dr-mbc pair 33
// of pool tail-01, seed 525772).
func TestMBCAbandonHopeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "xspell_discardx_unscripted")
}
