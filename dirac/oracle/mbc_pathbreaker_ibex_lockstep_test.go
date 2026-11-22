package oracle

import "testing"

// Pathbreaker Ibex ({4}{G}{G}) was offered as a cast by gorge but not by the
// kernel at p0's priority decision 363 (tail-55-c000 pair 77).
func TestMBCPathbreakerIbexReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "pathbreaker_ibex_cast_offer")
}
