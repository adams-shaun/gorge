package oracle

import "testing"

// Paradise Mantle's equip offer at priority after the Mantle is already
// attached (lockstep tail-26-c000 pair 90, decision 780): gorge and dirac must
// offer the same activations.
func TestMBCParadiseMantleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "paradise_mantle_equip_offer_after_equipped")
}
