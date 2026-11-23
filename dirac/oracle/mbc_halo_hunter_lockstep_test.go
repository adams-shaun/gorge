package oracle

import "testing"

// TestMBCHaloHunterReprosLockstep replays the recorded tail-51 game where
// gorge offered cast(Halo Hunter) at a priority decision and dirac did not.
func TestMBCHaloHunterReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "halo_hunter_cast_offer")
}
