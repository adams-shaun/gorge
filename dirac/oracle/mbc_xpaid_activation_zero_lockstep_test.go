package oracle

import "testing"

// Chamber Sentry, cast for X=1, activates "{X}, {T}, Remove X +1/+1 counters
// from among creatures you control: It deals X damage to any target" for
// X=0 at a Servo token. gorge's Count$xPaid fell back to the permanent's
// cast X whenever the ability's X was zero, so it dealt 1 and killed the
// Servo; the kernel dealt 0 (tail-25 pair 0, decision 404, Cards.Pos). An
// announced X, zero included, is now the ability's own (effects
// evalCountBodyCostXPaid).
func TestMBCChamberSentryXZeroReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "xpaid_activation_zero_not_cast_x")
}
