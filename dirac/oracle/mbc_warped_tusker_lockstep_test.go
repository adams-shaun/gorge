package oracle

import "testing"

// Warped Tusker's cycling offer (decision 640 of tail-57 pair 385, p0 holding
// the Tusker with Hall of Tagsin in play): gorge offered activate(Warped
// Tusker) and the kernel did not.
func TestMBCWarpedTuskerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cycling_warped_tusker_offer")
}
