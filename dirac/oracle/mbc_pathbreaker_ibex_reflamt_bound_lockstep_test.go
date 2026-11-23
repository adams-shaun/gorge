package oracle

import "testing"

// Pathbreaker Ibex ({4}{G}{G}, no restriction riders) was offered as a cast by
// gorge but not by the kernel at p1's priority decision 357 (tail-55 pair 26,
// surface full, mulligans 0, seed 595286): a second state of the reflected
// Amount$ class beside a plain cast, whose bound play the kernel's script
// offers only under the caAny gate (script_gen.go), not under ca.
func TestMBCPathbreakerIbexReflamtBoundReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflamt_bound_play_generic_cast_offer")
}
