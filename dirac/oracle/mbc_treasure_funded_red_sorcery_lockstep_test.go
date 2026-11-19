package oracle

import "testing"

// Destructive Tampering ({2}{R} sorcery) must be offered at a priority
// decision whose controller's only red is a sacrificed Treasure token
// (Sticky Fingers' combat-damage trigger on Kappa Tech-Wrecker): the
// same pool offers the {1}{R} Axgard Cavalry beside it, so the payment
// exists. dr-mbc-aabb pair 116 (seed 855460), decision 104: the kernel
// once dropped the cast option the treasure funds.
func TestMBCDestructiveTamperingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "treasure_funded_red_sorcery_not_offered")
}

// TestMBCDestructiveTamperingPoolGlob replays the pool's recordings by glob
// (repro_glob_test.go's table needs a test to glob each pool).
func TestMBCDestructiveTamperingPoolGlob(t *testing.T) {
	t.Parallel()
	stdReprosGlobPools(t, "dr-mbc-aabb")
}
