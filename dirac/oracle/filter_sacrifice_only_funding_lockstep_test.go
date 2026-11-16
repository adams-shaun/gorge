package oracle

import "testing"

// CR 605.1a, 601.2g: Hall of Tagsin's "{1}, {T}: Add one mana of any color"
// beside an Eldrazi Spawn token. The Spawn's sacrifice-only mana ability is a
// last-resort planner unit, not a census gap, so the filter is still the only
// gap and gorge's prefix search funds its {1} by sacrificing the Spawn: a
// Swamp's {B} and the filter's {B} cast Hunted Horror ({B}{B}). The kernel left
// the filter out of the search whenever a Spawn was on the board.
// The recording is on a custom pool, the table its own decks need: the tail
// buckets a synthetic pool deals re-cut whenever the lowered card set moves.
func TestFilterFeeFundedBySacrificeOnlySourceLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_fee_funded_by_sacrifice_only_source")
}
