package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

const (
	// wpSpell costs {B} plus {4} or a sacrificed artifact or creature.
	wpSpell = "Name:Witness Pool Spell\nManaCost:B\nTypes:Sorcery\n" +
		"K:AlternateAdditionalCost:Sac<1/Artifact;Creature/a creature or an artifact>:4\n" +
		"A:SP$ Destroy | ValidTgts$ Creature | SpellDescription$ Kill one creature.\nOracle:x\n"
	// wpAllyLand makes {C}, or one mana of any colour only an Ally spell may spend.
	wpAllyLand = "Name:Witness Pool Ally Land\nManaCost:no cost\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Colourless mana.\n" +
		"A:AB$ Mana | Cost$ T | Produced$ Any | RestrictValid$ Spell.Ally,Activated.Ally | SpellDescription$ Add one mana of any colour.\nOracle:x\n"
	// wpComboLand makes {B} or {R}: one mana, though the bound counts both colours.
	wpComboLand = "Name:Witness Pool Combo Land\nManaCost:no cost\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ Combo B R | SpellDescription$ Black or red mana.\nOracle:x\n"
	wpSwamp = "Name:Witness Pool Swamp\nManaCost:no cost\nTypes:Basic Land Swamp\nOracle:x\n"
	wpBear  = "Name:Witness Pool Bear\nManaCost:1\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
)

// wpCastPlan lists seat 0's potential cast of the spell in hand over lands.
func wpCastPlan(t *testing.T, lands ...string) PotentialPlan {
	t.Helper()
	e, _, _ := newFixtureDeck(t, 9404, ppFiller)
	spell := handCard(e, card(t, wpSpell), 0)
	onBoard(t, e, 1, wpBear)
	for _, l := range lands {
		onBoard(t, e, 0, l)
	}
	e.G.Step = state.StepMain1
	ppPriority(t, e)
	return ppFind(t, e.PotentialPaymentPlans(0), "cast", spell)
}

// A witness walk prices the offer against what the plan floats. With a
// restricted any-colour ability on the board the walk used to rebuild its pool from
// the whole board's bound, where a {B}/{R} land counts in both colours: four lands
// then looked like five mana and a {B} witness confirmed a cast that needs {B}{4}
// or a sacrifice nothing here can pay (CR 601.2b, 601.2f-h).
func TestPotentialWitnessWalkIgnoresTheBoardBoundBesideARestrictedAbility(t *testing.T) {
	t.Parallel()
	pp := wpCastPlan(t, wpSwamp, wpSwamp, wpAllyLand, wpComboLand)
	if pp.Plan != nil || pp.Reason == "" {
		t.Fatalf("four lands (%+v) offered the {B}+{4} cast as payable: %+v", pp.Reason, pp.Plan)
	}
}

// The same board with a fifth real mana still witnesses the cast.
func TestPotentialWitnessWalkStillConfirmsAPayableCastBesideARestrictedAbility(t *testing.T) {
	t.Parallel()
	pp := wpCastPlan(t, wpSwamp, wpSwamp, wpSwamp, wpAllyLand, wpComboLand)
	if pp.Plan == nil || pp.Reason != "" {
		t.Fatalf("five lands: verdict %+v, want a witness", pp)
	}
}
