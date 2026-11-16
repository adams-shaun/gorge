package oracle

import "testing"

// Ivory Cup's "you may pay {1}" trigger: gorge and dirac must offer the same
// pay/decline choice (dr tail-34-c002 pair 5, decision 613).
func TestMBCIvoryCupReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ivory_cup_trigger_cost_pay_offer")
}
