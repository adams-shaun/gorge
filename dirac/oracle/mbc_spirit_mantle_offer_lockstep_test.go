package oracle

import "testing"

// CR 601.2f: Strong Back's "Aura spells you cast that target enchanted
// creature cost {3} less" reads its ValidTarget$ (Creature.EnchantedBy) from
// the Strong Back's side. With the one untapped Plains, p1's Spirit Mantle
// ({1}{W}) is castable at the creature Strong Back enchants (the opponent's
// Stoneforge Mystic), so gorge offers the cast; dirac's offer probe matched
// the filter with no source and priced the Mantle at its full cost, leaving
// the cast out (tail-07-c000 pair 5, decision 136 of 136).
func TestMBCSpiritMantleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cost_reduction_target_enchanted_by_source")
}
