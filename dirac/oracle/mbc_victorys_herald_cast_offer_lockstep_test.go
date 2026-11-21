package oracle

import "testing"

// Victory's Herald ({3}{W}{W}{W}) is offered by exactly one of the two engines at
// priority (decision 729 of tail-08-c000 pair 13).
func TestMBCVictorySHeraldReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "victorys_herald_cast_offer")
}
