package oracle

import "testing"

// A creature dying under Vigil for the Lost asks its controller for the paid X.
// With one untapped Viridescent Bog ({1},{T}: add {B}{G}) and nothing else to
// pay with, gorge offered X up to 3 while dirac stopped at 2
// (ev-tail-12-c000 pair 148, decision 1232).
func TestMBCSuppressorSkyguardReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "vigil_paid_x_ceiling")
}
