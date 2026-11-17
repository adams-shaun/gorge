package oracle

import "testing"

// CR 601.2f, 605.3b: Goblin Fireslinger ({R}) is castable from a board whose
// only red source is Grove of the Burnwillows' rider ability; gorge offers the
// cast and the kernel once did not (decision 162 of tail-04 pair 8).
func TestMBCGoblinFireslingerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "free_rider_land_cast_offer")
}
