package oracle

import "testing"

// Dust Elemental ({3}{W}{W}{W}) was offered as a cast by gorge but not by dirac
// at a priority decision (decision 596 of tail-34 pair 5).
func TestMBCDustElementalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dust_elemental_cast_offer")
}
