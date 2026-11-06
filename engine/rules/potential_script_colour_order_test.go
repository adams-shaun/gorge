package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// The Great Divide Guide board prices a 4GG cast through the granted
// any-colour mana abilities, whose colour asks gorge orders with
// scriptOrderAnswers: the play's G pips first. The search is budget-capped,
// so on this board the needed G is reached within a handful of clones only
// when the colour ask is reordered; in engine order (W U B R G) the same
// budget is spent proving W/U/B/R dead.
//
// The budget is the assertion: with the reorder the script is found at 10
// clones, without it even 14 clones find nothing (the pin this test exists
// for). The fixture itself is the census shape of
// TestGuideGrantCensusOffersSixManaCast; this test is the ordering regression.
func TestScriptColourAskPutsNeededColourFirst(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9404, ggcFlopsie)
	toMain1(t, e)
	onBoardReady(t, e, 0, ggcGuide)
	onBoardReady(t, e, 0, ggcGran)
	onBoard(t, e, 0, ggcForest())
	for i := 0; i < 4; i++ {
		onBoard(t, e, 0, ggcIsland())
	}
	ids := setHandSrcs(t, e, 0, ggcFlopsie)
	ppPriority(t, e)

	a := decision.PotentialAction{Kind: "cast", Obj: ids[0]}
	script, reason := e.PotentialPlayScript(0, a, 10)
	if reason != "" {
		t.Fatalf("search found no script within 10 clones (reason %q); the colour ask is not putting the cast's G first", reason)
	}
	if len(script) == 0 {
		t.Fatal("search reported a script but it is empty")
	}
	gotG := false
	for _, st := range script {
		for _, p := range st.Picks {
			if p.ManaSymbol == "G" {
				gotG = true
			}
		}
	}
	if !gotG {
		t.Fatalf("script %+v never answered a colour ask G, though the cast announces G G", script)
	}
}
