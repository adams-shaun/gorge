package oracle

import "testing"

// Dragon Tyrant's upkeep trigger ("sacrifice unless you pay {R}{R}{R}{R}") posed a modes
// decision for p1 where the kernel offered mode(-,-,0) and mode(-,-,1) and gorge only
// mode(-,-,0) (decision 607 of tail-16-c000 pair 109).
func TestMBCDragonTyrantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_pay_upkeep_mode_unaffordable")
}
