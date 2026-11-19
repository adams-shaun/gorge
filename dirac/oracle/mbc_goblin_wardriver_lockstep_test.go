package oracle

import "testing"

// Goblin Wardriver's {R}{R} beside Starfleet Academy's {5},{T} (two creature
// types make RR) and a Three Tree City whose chosen type matches no creature.
// The land's {2},{T} makes nothing and gorge's census drops it, but its
// activation as a funding source still poses the ability ask and the filter's
// colour ask in gorge's prefix search, and those dead branches spend the
// 160-node budget before the Academy line is reached, so gorge never offers the
// cast. The kernel's mirror walked the {C} answer alone, found the line in 95
// nodes, and offered it (decision 445 of 445; dr-mbc-abbb pair 29 before the
// MBC split).
func TestMBCGoblinWardriverReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "zero_dynamic_filter_funder_spends_search_budget")
}
