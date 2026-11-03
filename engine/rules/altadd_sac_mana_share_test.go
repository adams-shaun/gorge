package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	// asmSpell costs {W} and a sacrificed artifact or creature, as one additional cost.
	asmSpell = "Name:Shared Fodder Spell\nManaCost:W\nTypes:Artifact\n" +
		"K:AlternateAdditionalCost:Sac<1/Artifact;Creature/an artifact or creature>\n" +
		"Oracle:x\n"
	// asmCoin is an artifact that sacrifices itself for one mana of any colour.
	asmCoin = "Name:Shared Fodder Coin\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ T Sac<1/CARDNAME/this artifact> | Produced$ Any | Amount$ 1 | SpellDescription$ Add one mana of any colour.\nOracle:x\n"
	asmIsland = "Name:Shared Fodder Island\nManaCost:no cost\nTypes:Basic Land Island\nOracle:x\n"
)

// An additional cost that sacrifices an artifact or creature is paid beside the
// spell's mana (CR 601.2b, 601.2h): when the only permanent it can sacrifice is
// the one source of the colour the spell needs, which sacrifices itself for that
// mana (CR 601.2g), the cast cannot be completed and is not offered.
func TestAltAddSacrificeIsNotTheOnlyManaSource(t *testing.T) {
	t.Parallel()
	plans := func(extraCoin bool) bool {
		e, _, spell := newFixtureDeck(t, 9411, asmSpell)
		onBoard(t, e, 0, asmCoin)
		onBoard(t, e, 0, asmIsland)
		if extraCoin {
			onBoard(t, e, 0, asmCoin)
		}
		e.G.Step = state.StepMain1
		ppPriority(t, e)
		got := e.potentialModeCastPlan(0, decision.Option{Kind: "cast", Obj: spell})
		return got.Plan != nil && got.Reason == ""
	}
	if plans(false) {
		t.Fatalf("the cast was planned with the coin both its only {W} and its only sacrifice")
	}
	if !plans(true) {
		t.Fatalf("the cast was not planned with a second artifact to sacrifice")
	}
}
