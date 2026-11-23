package oracle

import "testing"

// Colos Yearling's {R}: +1/+0 ability: gorge offered activate(Colos Yearling)
// at a priority decision where the kernel offered only pass (decision 468 of
// tail-41-c000 pair 1, autopay).
func TestMBCColosYearlingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "colos_yearling_pump_offer")
}
