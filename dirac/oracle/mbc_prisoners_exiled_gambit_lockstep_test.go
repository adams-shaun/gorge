package oracle

import "testing"

func TestMBCPrisonersExiledGambitReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "prisoners_exiled_gambit_offer")
}
