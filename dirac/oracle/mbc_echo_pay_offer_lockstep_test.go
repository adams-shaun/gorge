package oracle

import "testing"

// CR 702.30a: Stingscourger's echo trigger asks its controller to pay {3}{R}
// or sacrifice it. Only one engine offered the pay option (decision 98 of
// tail-18-c000 pair 109).
func TestMBCStingscourgerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "echo_pay_offer")
}
