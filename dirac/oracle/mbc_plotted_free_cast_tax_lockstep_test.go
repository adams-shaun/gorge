package oracle

import "testing"

// CR 601.2f with CR 702.170d: a plotted card is cast without paying its mana
// cost, but cost increases still apply to that total. p1 plotted Lock and Load
// and, two turns later, tapped every mana source in upkeep, so with Thalia,
// Guardian of Thraben's {1} tax on noncreature spells the plotted cast costs
// {1} they can't pay. gorge offers only pass; dirac offered the plotted cast
// (decision 604; tail-07 pair 150, autopay, seed 207208).
// The recording is on a custom pool, the table its own decks need: the tail
// buckets a synthetic pool deals re-cut whenever the lowered card set moves.
func TestMBCLockAndLoadReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "plotted_free_cast_pays_cost_increase")
}
