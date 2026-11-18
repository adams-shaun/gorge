package oracle

import (
	"testing"
)

// CR 616.1: two Maze of Ith activations on one attacking creature (one by
// each player, this turn) leave two prevention Effects live, and beside a
// Jaya, Venerated Firemage's damage-plus raise each is its own candidate of
// the CR 616.1 order ask: three options. The kernel recorded only one
// prevention grant per creature (the last grant's source), so its menu had
// two options and parted at decision 292 (pool tail-57 pair 73; the
// TestMBCMazeOfIthReprosLockstep name is taken by the shield-ask repro).
func TestMazeMultiPrevGrantsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "maze_multi_prev_grants")
}
