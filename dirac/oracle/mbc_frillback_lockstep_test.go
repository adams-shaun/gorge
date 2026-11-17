package oracle

import "testing"

// Tranquil Frillback's "pay {G} up to three times" announcement: the evidence
// loop's pool tail-35-c005 pair 15 parted at decision 225 (gorge offered the
// counts 0..3, the kernel only the decline).
//
// CR 117.3b / 601.3b: when a trigger's cost is optional ("you may pay {G} up
// to three times"), the cost payment itself is a choice the player is offered
// alongside declining. At decision 132 of tail-67-c000 pair 87, gorge offered
// p1 both the pay and the decline option for Tranquil Frillback's trigger
// cost (Fountain of Cho's indeterminate {W} priced unbounded in every colour,
// CR 106.1); the kernel offered only the decline.
func TestMBCTranquilFrillbackReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "frillback_announce_count_unpayable")
	stdReproNamed(t, "frillback-trigger-cost-pay")
}
