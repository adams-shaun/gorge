package oracle

import "testing"

// CR 704.5m with CR 702.16: an opponent's red Aura (Frenzied Rage) on Empty-Shrine
// Kannushi stays legal while its controller controls no red permanent, and falls
// off the moment one arrives (Ox of Agonas). Gorge's state-based-action quiet skip
// treated that plain entry as inert and kept the Aura (decision 297 of 297, tail-06
// pair 56, the first divergence "projection Cards[9].Power").
func TestMBCSkeletalSwarmingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "colors_you_ctrl_bearer_drops_opposing_aura_when_a_permanent_enters")
}
