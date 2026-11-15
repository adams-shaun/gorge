package oracle

import "testing"

// Avarice Amulet's equip activation is offered by gorge at decision 604 of
// pool tail-17 pair 232 and not by dirac's priority offer.
func TestMBCAvariceAmuletReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "avarice_amulet_equip_offer")
}
