package oracle

import "testing"

const krosanDruidRepro = "kicked_cast_gap_search_budget_exhausted"

// CR 702.33a, 605.3: Krosan Druid's kicked cast costs {6}{G}{G}. With a
// floating {U}{U}{U}{U}{G}, six Islands under Scald, four Forests and
// Overflowing Basin's {1},{T} filter untapped, gorge's potential-play prefix
// search for that cast walks its whole potentialScriptNodes budget (plain,
// phased and subsume runs) and prices nothing, so the autopay surface withholds
// the kicked cast; the kernel's mirror of the search finds a payment and offers
// it (tail-49-c000 pair 25, decision 861, "options options"). That was the
// divergence the card was refused for. At the recorded decision both engines
// now offer the casts of Krosan Druid, and the recorded game replays clean on a
// table of its own cards, so the card is unrefused with this game as its proof
// (dirac/support/unrefused.tsv).
func TestMBCKrosanDruidReprosLockstep(t *testing.T) {
	t.Parallel()
	oneOffReproClean(t, krosanDruidRepro)
}
