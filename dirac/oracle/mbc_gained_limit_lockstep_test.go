package oracle

import "testing"

const patchworkCrawlerGainedLimitRepro = "patchwork_crawler_gained_once_each_turn_ability_unmodelled"

// CR 613.1f with CR 602.5b: Patchwork Crawler has the activated abilities of the
// creature cards exiled with it, so Stromkirk Condemned's "activate only once
// each turn" pump becomes the Crawler's, limit and all. The GainsOf walk now
// offers the gained once-each-turn ability once per (recipient, foreign object,
// index) per turn (dirac/static_gainsof.go gainedOffer, markGainedUsed), so the
// recorded tail-03-c001 pair 54 game replays clean and the Crawler is no longer
// refused (dirac/support/unrefused.tsv).
func TestMBCTail03ReprosLockstep(t *testing.T) {
	t.Parallel()
	res := replayRawRepro(t, patchworkCrawlerGainedLimitRepro)
	if res.Div != nil {
		t.Fatalf("%s's recorded game diverges at %d: %s", patchworkCrawlerGainedLimitRepro, res.Div.At, res.Div)
	}
	if res.StallUnmodelled {
		t.Fatalf("%s's recorded game ends at an unmodelled stall: %q", patchworkCrawlerGainedLimitRepro, res.Stall)
	}
}
