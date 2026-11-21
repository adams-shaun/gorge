package oracle

import "testing"

// Evidence-loop pair (pool tail-73, seed 647, pair 8): at decision 605 p1's
// priority offered gorge cast(Deepglow Skate) while the kernel offered
// nothing for it (options options). The kernel side was repaired by the
// filter-gap and restricted-census-gap work that landed after main 5c4aea4081
// (53d0604b58, 2d110d1e35); this recording pins the whole game so the cast
// stays offered by both engines.
func TestMBCDeepglowSkateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "deepglow_skate_cast_offer")
}
