package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// ppSquire is a white creature: its colour is one Bloom Tender adds.
const ppSquire = "Name:Potential Squire\nManaCost:W\nTypes:Creature Human\nPT:1/1\nOracle:x\n"

// TestPaymentPlanPricesAnEachColourSource: Bloom Tender's "one mana of each
// colour among permanents you control" is a fixed production read off the
// board, {W}{G} beside a white creature, so the planner prices it as one
// source (CR 106.1a, 605.1a). A {W}{G}{G} spell over the Tender, the
// creature and a Forest has a two-tap witness and no scripted prefix, the
// witness's taps make exactly what they record, and a {W}{W} spell is a
// proof of insufficiency, not an incomplete census.
func TestPaymentPlanPricesAnEachColourSource(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		cost string
		taps int
	}{{"W G G", 2}, {"W W", 0}} {
		e, _, spell := newFixtureDeck(t, 9431, "Name:Potential Batch\nManaCost:"+tc.cost+"\nTypes:Instant\nA:SP$ Draw | NumCards$ 1\nOracle:x\n")
		onBoardReadyCard(t, e, 0, corpusCard(t, "Bloom Tender"))
		onBoard(t, e, 0, ppSquire)
		onBoard(t, e, 0, lrForest)
		ppPriority(t, e)
		ppCheckWitness(t, e, spell, tc.taps)
		pp := ppFind(t, e.PotentialPaymentPlans(0), "cast", spell)
		if len(pp.Script) != 0 {
			t.Fatalf("{%s}: a scripted prefix %+v, want the planner's plan", tc.cost, pp.Script)
		}
		if tc.taps != 0 && !e.potentialWitnessReaches(0, decision.Option{Kind: "cast", Obj: spell}, *pp.Plan) {
			t.Fatalf("{%s}: the witness does not reach the cast", tc.cost)
		}
	}
}
