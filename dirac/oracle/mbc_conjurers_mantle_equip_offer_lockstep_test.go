package oracle

import "testing"

// CR 702.6a: Conjurer's Mantle's Equip {1} is offered at decision 918 of
// tail-34-c000 pair 26 by dirac but not by gorge.
func TestMBCConjurerSMantleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "conjurers_mantle_equip_offer")
}
