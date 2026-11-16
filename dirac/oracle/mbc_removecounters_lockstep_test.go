package oracle

import "testing"

// Chandra, Legacy of Fire's "remove a loyalty counter from each of any number
// of permanents you control" (gorge removeCounterChoose: Min 0, Max the whole
// eligible pool) with more than sixteen permanents. The kernel closed the
// pick at its sixteenth, so gorge's seventeenth counter_pick ask had no
// kernel twin (decision 965 of tail-58 pair 3, seed 1).
func TestMBCTail58ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "remove_counters_any_number_past_sixteen")
}
