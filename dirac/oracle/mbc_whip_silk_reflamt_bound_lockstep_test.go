package oracle

import "testing"

// Whip Silk ({1}{G}) was offered as a cast by gorge but not by the kernel at
// p1's priority decision 730 (tail-55 pair 43, surface full, mulligans 0, seed
// 595286): the same reflected Amount$ bound-play gate as the Ibex pair, and
// the same caAny fix closes it.
func TestMBCWhipSilkReflamtBoundReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflamt_bound_play_aura_cast_offer")
}
