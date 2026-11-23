package oracle

import "testing"

// CR 702.27a with CR 605.3: Capsize's buyback cast ({1}{U}{U}+{3}) is payable
// only through every source at once -- Island funding Cascade Bluffs' {U/R}
// filter and Mage-Ring Network's two storage counters. gorge's prefix search
// spends its 160-node budget on that board and leaves the play
// census_incomplete (dropped by the autopay surface), so only the kernel
// offered the cast (decision 308 of tail-55 pair 9, seed 176622).
func TestMBCCapsizeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "buyback_cast_filter_store_search_budget")
}
