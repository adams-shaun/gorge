package oracle

import "testing"

// Grim Physician ({B}) is offered by gorge at a priority decision where dirac
// leaves it out (decision 745, dr tail-50 pair 32).
func TestMBCGrimPhysicianReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grim_physician_cast_offer")
}
