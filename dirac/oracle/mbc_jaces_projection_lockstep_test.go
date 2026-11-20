package oracle

import "testing"

// Jace's Projection {2}{U}{U} and Soratami Savant {2}{U}{U} were offered by
// gorge at a priority decision where the kernel offered only the Pip-Boy 3000
// equip (tail-49-c001 pair 2, decision 577 of 577).
func TestMBCJaceSProjectionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "jaces_projection_cast_offer_after_fee_land")
}
