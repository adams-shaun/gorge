package oracle

import "testing"

// Pursued Whale's target offer at decision 624 of the tail-28-c000 pair 94
// recording: gorge offered #23{Pursued Whale} as a target where dirac did not.
func TestMBCPursuedWhaleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "pursued_whale_target_offer")
}
