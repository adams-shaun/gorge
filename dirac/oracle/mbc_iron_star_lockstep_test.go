package oracle

import "testing"

// Iron Star's "you may pay {1}" trigger cost: gorge offered trigger_cost_pay
// beside trigger_cost_decline where the kernel offered decline alone
// (decision 1143 of tail-71-c000 pair 9, autopay).
func TestMBCIronStarReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "iron_star_trigger_cost_pay_offer")
}
