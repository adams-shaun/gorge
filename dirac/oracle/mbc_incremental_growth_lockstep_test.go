package oracle

import "testing"

// Incremental Growth's three TargetUnique$ slots: gorge offered the cast at
// decision 576 of the tail-42 pair 8 game and dirac did not.
func TestMBCIncrementalGrowthReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "incremental_growth_three_distinct_targets")
}
