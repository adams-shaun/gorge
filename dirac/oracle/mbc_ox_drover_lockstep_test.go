package oracle

import "testing"

const oxDroverRepro = "ox_drover_prismite_ogre_offer"

// CR 605.3b with CR 119.4: Ox Drover's {3}{W} cast (tail-51-c000 pair 166,
// decision 368) is payable only when Treasonous Ogre's "Pay 3 life: Add {R}"
// is activated three times beside the one untapped Plains, at 13 life, with a
// Prismite on the board. Prismite's {2} fee-only filter leaves gorge's census
// unrelaxable, so its prefix search prices every insufficient play and offers
// the cast. The kernel's filter search once spent a sole life-only pain unit
// once and never offered it; it now counts the unit's activation as a gap use
// and keeps a tapped life-only unit funding (dirac/mana_filter.go lifePainBranch,
// slotUnusable), so on the pinned game both engines offer the cast and the
// recorded game replays clean. The card is therefore no longer refused
// (dirac/support/unrefused.tsv); this test pins the unrefusal's proof.
func TestMBCOxDroverReprosLockstep(t *testing.T) {
	res := replayRawRepro(t, oxDroverRepro)
	if res.Div != nil {
		t.Fatalf("%s's recorded game diverges at %d: %s", oxDroverRepro, res.Div.At, res.Div)
	}
	if res.StallUnmodelled {
		t.Fatalf("%s's recorded game ends at an unmodelled stall: %q", oxDroverRepro, res.Stall)
	}
}
