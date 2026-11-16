package oracle

import "testing"

// CR 117.3b / 601.3b: when a trigger's cost is optional ("you may pay {G} up
// to three times"), the cost payment itself is a choice the player is offered
// alongside declining. At decision 132 of tail-67-c000 pair 87, gorge offered
// p1 both the pay and the decline option for Tranquil Frillback's trigger
// cost; the kernel offered only the decline.
func TestMBCTranquilFrillbackReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "frillback-trigger-cost-pay")
}
