package oracle

import "testing"

// TestMBCSpawningBedReprosLockstep replays the tail-14 game whose first
// divergence was an options decision at priority: gorge offered Spawning Bed's
// "{6}, {T}, Sacrifice Spawning Bed: Create three 1/1 colorless Eldrazi Scion
// creature tokens" activation and the kernel did not. The recording pins the
// activation's offer and payment (CR 602.2, 601.2g-h).
func TestMBCSpawningBedReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spawning_bed_options")
}
