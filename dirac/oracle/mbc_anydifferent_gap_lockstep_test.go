package oracle

import "testing"

// Firemind Vessel's "{T}: Add two mana of different colors" (Produced$ Combo
// AnyDifferent) is a census gap in gorge's payment planner: the production
// resolves to no mana ("unhandled Produced$ AnyDifferent"), but gorge's prefix
// search (scriptSearch.visit) activates every uncovered source in battlefield
// order, so the Vessel is tapped for nothing ahead of the real gap that funds
// the play. Pool tail-43 pair 300 (also pairs 71, 217, 298 and 307): Crimson
// Muckwader's {2}{B} regeneration at decision 938, paid by a Survivors'
// Encampment tapping a creature for black, the untapped Vessel earlier in
// battlefield order than the Encampment. The kernel's tap-another search left
// it untapped (Cards[13].Flags: ref=1 kernel=0).
//
// The same census gap in the two other windows: a Mystic Remora's pay-{4}
// unless ask lists the Vessel in battlefield order and taps it for nothing
// (pair 217; windowGapListed took it only in a trigger-cost window), and a
// ward {1} payment opens its window while an untapped Vessel is the only source
// left, tapping it before the payment fails (seed 3 pair 103; tapAllForMana
// skipped it).
//
// The recordings replay on pool custom, the table their own decks build: the
// tail buckets shift as kgen lowers more cards.
func TestFVProjectionFlagsReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"anydifferent_gap_tapped_by_prefix_search",
		"anydifferent_gap_tapped_by_unless_window",
		"anydifferent_gap_tapped_by_ward_payment",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stdReproNamed(t, name)
		})
	}
}
