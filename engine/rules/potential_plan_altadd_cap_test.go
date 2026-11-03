package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

const (
	// acSpell costs {B} plus {4} or a sacrificed artifact or creature.
	acSpell = "Name:Cap Spell\nManaCost:B\nTypes:Sorcery\n" +
		"K:AlternateAdditionalCost:Sac<1/Creature;Artifact/artifact or creature>:4\n" +
		"A:SP$ Destroy | ValidTgts$ Creature | SpellDescription$ Destroy target creature.\nOracle:x\n"
	// acGranter gives every land its controller has a tap for one mana of any colour.
	acGranter = "Name:Cap Granter\nManaCost:1\nTypes:Creature Scout\nPT:1/1\n" +
		"S:Mode$ Continuous | Affected$ Land.YouCtrl | AddAbility$ CapAny | Description$ Lands tap for any colour.\n" +
		"SVar:CapAny:AB$ Mana | Cost$ T | Produced$ Any | Amount$ 1 | SpellDescription$ Add one mana of any colour.\nOracle:x\n"
	acForest = "Name:Cap Forest\nManaCost:no cost\nTypes:Basic Land Forest\nOracle:x\n"
)

// The prefix search that taps a granted mana ability first (potentialScriptPlays
// of them per query) is spent on an either-or additional cost's cast too: with one
// such cast more than the cap, each payable only by the granted {B}, the walk
// prices the first potentialScriptPlays with a script and leaves the last unpriced.
func TestPotentialScriptCapCountsAnEitherOrAdditionalCostCast(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9404, ppFiller)
	var spells []state.ObjID
	for range potentialScriptPlays + 1 {
		spells = append(spells, handCard(e, card(t, acSpell), 0))
	}
	onBoard(t, e, 1, wpBear)
	onBoardReady(t, e, 0, acGranter)
	onBoard(t, e, 0, acForest)
	e.G.Step = state.StepMain1
	ppPriority(t, e)
	pps := e.PotentialPaymentPlans(0)
	priced := 0
	for i, id := range spells {
		pp := ppFind(t, pps, "cast", id)
		if (pp.Script != nil) != (i < potentialScriptPlays) {
			t.Errorf("cast %d of %d: script %v, verdict %q", i, len(spells), pp.Script != nil, pp.Reason)
		}
		if pp.Script != nil {
			priced++
		}
	}
	if priced != potentialScriptPlays {
		t.Fatalf("%d casts priced, want %d", priced, potentialScriptPlays)
	}
}
