package oracle

import "testing"

// Ichorid's upkeep return costs "exile a black creature card other than
// Ichorid from your graveyard"; gorge offered only the decline while the
// kernel offered pay too (decision 444, dr evidence loop tail-73).
func TestMBCIchoridReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ichorid_upkeep_exile_cost")
}
