package oracle

import "testing"

// Gather Courage ({G}, Convoke) is cast at priority in gorge's offers but not
// in dirac's (decision 389 of dr tail-09 pair 172).
func TestMBCGatherCourageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "gather_courage_convoke_cast_offer")
}
