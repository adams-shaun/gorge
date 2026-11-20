package oracle

import "testing"

// CR 602.2, 601.2g: Abandoned Air Temple's {3}{W},{T} activation on a board
// whose only white source is White Lotus Hideout's {1},{T} filter. gorge does
// not offer the activation; the kernel offered it (decision 632, std-tla pair
// 79).
func TestMBCAbandonedAirTempleActivateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "abandoned_air_temple_activate_unfunded")
}
