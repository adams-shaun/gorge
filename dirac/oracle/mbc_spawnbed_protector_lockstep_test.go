package oracle

import "testing"

// Spawnbed Protector's priority cast was once offered by one engine only
// (`options options` at decision 509 of tail-41-c000 pair 9, seed 899524).
// The game now passes that decision and ends at the declared landgrant
// kernel stall (decision 563), so the recording replays without a divergence.
func TestMBCSpawnbedProtectorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spawnbed_protector_priority_cast")
}
