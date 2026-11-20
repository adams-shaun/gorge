package oracle

import "testing"

// CR 605.3a with CR 601.2g: Summoning Station's cast is a plain {7}, so
// Prismite's "{2}: Add one mana of any color" fills no fixed pip the cast still
// owes after the fee — spending {2} for one mana buys nothing the pool could
// not already make — and gorge's potential script skips the fee-only filter's
// colour ask (scriptSearch.feeOnlyAskPrune). The kernel once walked all five of
// the filter's colours at every grant line and spent its node budget before the
// grant plan, so its priority list omitted the cast of Summoning Station
// (decision 589 of 589; ev-tail-51-c000 pair 30, seed 77888).
func TestMBCSummoningStationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "doomed_fee_only_colour_walk")
}
