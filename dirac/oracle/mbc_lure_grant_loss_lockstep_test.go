package oracle

import "testing"

// CR 613.7g + 613.1f: Noble Quarry bestowed on a Construct token grants the
// quoted lure ability ("All creatures able to block this creature do so");
// Poppet Stitcher's back face, transformed after the Aura attached, makes the
// tokens lose all abilities, and the later loss ends the lure grant. gorge
// once kept it as a "rule effect" and offered p0's creatures only the lure
// blocks (decision 1019 of tail-13-c000 pair 47: gorge offered 2 blocks, the
// kernel 6 blocks plus done).
func TestMBCOrderOfTheGoldenCricketReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lure_grant_lost_to_later_transformed_loss")
}
