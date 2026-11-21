package oracle

import "testing"

// Faceless Butcher's cast is offered by gorge at a priority decision where dirac
// offers no cast (tail-55-c000 pair 5, decision 576).
func TestMBCFacelessButcherReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "faceless_butcher_cast_offer")
}
