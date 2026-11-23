package oracle

import "testing"

// Staff of Domination's priority decision at 823 of tail-36-c000 pair 47: gorge
// offered activate(#85{token}) and dirac did not.
func TestMBCStaffOfDominationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "staff_of_domination_token_activation_offer")
}
