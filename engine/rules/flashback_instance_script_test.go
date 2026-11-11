package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A card in the graveyard printing an expensive flashback, with a further
// instance granted at its cheaper mana cost (CR 702.34a: each instance is its
// own permission, cast for its own cost). The granted any-colour abilities
// leave the census incomplete, so the printed instance's verdict comes from
// the prefix search. Tapping five sources makes the cheaper instance castable,
// but that is a different option: the printed {8}{R} instance, the one the
// seat's matcher selects, is still out of reach, so no prefix prices it.
func TestPrefixScriptKeepsFlashbackInstanceApart(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const raid = "Name:Ash Column Raid\nManaCost:4 R\nTypes:Instant\nA:SP$ Draw | NumCards$ 1\nK:Flashback:8 R\nOracle:x\n"
	e, _, card := newFixtureDeck(t, 9705, raid)
	toMain1(t, e)
	onBoardCard(t, e, 0, tlaCorpusCard(t, reg, "Iroh, Grand Lotus"))
	onBoardReady(t, e, 0, ggcGuide)
	for i := 0; i < 4; i++ {
		onBoard(t, e, 0, "Name:FIS Mountain\nTypes:Land Mountain\nOracle:x\n")
	}
	toGraveyard(t, e, card)
	if x := e.extraFlashbackCosts(card); len(x) != 1 {
		t.Fatalf("precondition: extraFlashbackCosts = %v, want the granted mana-cost instance", x)
	}
	ppPriority(t, e)
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind == "cast" && pp.Action.Obj == card && pp.Reason == "" {
			t.Fatalf("flashback cast priced: script %+v; five mana cannot pay the printed {8}{R}", pp.Script)
		}
	}
}
