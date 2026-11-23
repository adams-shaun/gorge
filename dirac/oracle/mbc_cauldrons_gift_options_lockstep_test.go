package oracle

import "testing"

const cauldronsGiftOptionsRepro = "cauldrons_gift_options"

// Cauldron's Gift ({4}{B}, costing a card: it exiles a creature card from the
// graveyard) sits in hand beside Energy Refractor and Vivid Meadow, whose
// counter-cost taps pay the cast (pool tail-02 pair 91, seed 506928, decision
// 433). The plain census keeps the counter-tap ability out (prepCensusGap's
// ctrAb read, availCtr's units), so a cast only that gap's unit pays is priced
// over the extended census, while the filter search's fee branches spend its
// node budget before its answer: only gorge offered
// cast(#43{Cauldron's Gift},-,0) at the priority decision. Taking the offer
// side's second query (scriptOffers' filt branch) for the printed-cost cast
// the counter gap is load-bearing for -- the census without it cannot pay the
// cast and the one with it can -- lets both engines offer the cast, and the
// recorded game replays clean.
func TestMBCCauldronSGiftReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, cauldronsGiftOptionsRepro)
}
