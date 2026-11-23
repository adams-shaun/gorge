package oracle

import "testing"

// Phantasmagorian ({5}{B}{B}) was castable for gorge but missing from the
// kernel's priority options (tail-23-c000, decision 977 of pair 3).
func TestMBCPhantasmagorianReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phantasmagorian_cast_offer")
}
