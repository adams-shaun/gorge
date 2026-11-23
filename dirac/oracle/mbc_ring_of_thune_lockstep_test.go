package oracle

import "testing"

// CR 704.5n: an Equipment attached to a permanent that left the battlefield
// becomes unattached at the next state-based check, so the equipped creature's
// death and the Equipment's detach are two checks. Ring of Thune's equipped
// Kessig Recluse dies to combat damage in the same check that Abdel Adrian's
// exile-until-it-leaves returns land (tail-07-c000 pair 23, decision 811 of
// the loop's game); the two engines must show the same AttachedTo at the ask
// that follows.
func TestMBCRingOfThuneReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ring_of_thune_host_dies_before_ask")
}
