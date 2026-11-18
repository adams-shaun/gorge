package oracle

import "testing"

// CR 702.145d: "any time" a player controls a daybound permanent with neither
// day nor night, it becomes day; this isn't a state-based action. Harvesttide
// Infiltrator (3/2, daybound) resolves under the opponent's Elesh Norn, Grand
// Cenobite and has 0 toughness: the SBA pass that puts it into the graveyard
// is the same one that sees it on the battlefield, so it still becomes day.
// dirac once applied the death before its day/night check and stayed neither
// (decision 1519 of tail-36 pair 20, seed 552030).
func TestMBCHarvesttideInfiltratorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "daybound_enters_and_dies_makes_day")
}
