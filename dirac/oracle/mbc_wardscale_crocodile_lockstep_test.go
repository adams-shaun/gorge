package oracle

import "testing"

// Wardscale Crocodile ({4}{G}, Hexproof) was offered by the kernel at a priority
// decision where gorge withheld it (decision 721 of tail-08 pair 20, seed 933465).
func TestMBCWardscaleCrocodileReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "wardscale_crocodile_cast_offer")
}
