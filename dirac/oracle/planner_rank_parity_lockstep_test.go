package oracle

import "testing"

// TestPlannerRankParityReprosLockstep replays the two planner-rank-parity
// recordings (blk-a04): dirac's payment planner must rank sources the way
// gorge's does, so a filter land's {1},{T} extra alternative
// (planner_rank_parity: gorge paid a cast with an Island where dirac used
// Surveillance Room) and two identical basics (planner_rank_parity_b: gorge
// tapped two of three Mountains, dirac a different pair) never split the
// engines.
func TestPlannerRankParityReprosLockstep(t *testing.T) {
	t.Run("planner_rank_parity", func(t *testing.T) {
		t.Parallel()
		stdReproNamed(t, "planner_rank_parity")
	})
	t.Run("planner_rank_parity_b", func(t *testing.T) {
		t.Parallel()
		stdReproNamed(t, "planner_rank_parity_b")
	})
}
