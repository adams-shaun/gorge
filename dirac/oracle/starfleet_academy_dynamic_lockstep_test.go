package oracle

import "testing"

const starfleetAcademyPaidDynamicRepro = "paid_dynamic_mana_source_cast_offer"

// CR 601.2g, 605.3b: Starfleet Academy's {5},{T} "add X mana of one chosen
// colour" is a paid dynamic-amount filter. The kernel's filter search used to
// drop its free tap's dead subtree (freeBranchHopeless) where gorge's plain
// prefix run walks it and spends its node budget, so the kernel offered a cast
// gorge withholds. freeBranchHopeless now keeps the free branch for a dynamic
// production amount (dirac/mana_filter.go), so the recorded game replays clean
// and the card is no longer refused (dirac/support/unrefused.tsv).
func TestStarfleetAcademyPaidDynamicCastOfferReplaysClean(t *testing.T) {
	res := replayRawRepro(t, starfleetAcademyPaidDynamicRepro)
	if res.Div != nil {
		t.Fatalf("%s's recorded game diverges at %d: %s", starfleetAcademyPaidDynamicRepro, res.Div.At, res.Div)
	}
	if res.StallUnmodelled {
		t.Fatalf("%s's recorded game ends at an unmodelled stall: %q", starfleetAcademyPaidDynamicRepro, res.Stall)
	}
}
