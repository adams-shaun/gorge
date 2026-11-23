package oracle

import "testing"

// TestMBCForgottenHarvestReprosLockstep replays the shrunk game in which dirac
// offered cast(Forgotten Harvest) at priority and gorge did not.
func TestMBCForgottenHarvestReprosLockstep(t *testing.T) {
	stdReproNamed(t, "forgotten_harvest_cast")
}
