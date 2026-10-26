package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// legendCrest is The Ringhart Crest's mana ability alone: green spendable
// only on a legendary creature spell.
const legendCrest = "Name:Legend Crest\nTypes:Artifact\nA:AB$ Mana | Cost$ T | Produced$ G | RestrictValid$ Spell.Creature+ChosenType,Spell.Creature+Legendary | SpellDescription$ x\nOracle:x\n"

// TestRestrictedSourceRefusesModalBackFace: a Modal DFC's back face is cast
// with that face's characteristics (CR 712.11, 712.4d). Kolvori, God of
// Kinship is a legendary creature, The Ringhart Crest a legendary artifact,
// so green restricted to legendary creature spells cannot pay the Crest's
// {1}{G}: with only that source and a Swamp untapped, the back face is not
// offered (cr-g125 priced the restriction against the hand card's front
// face in the potential planner, whose witness then named a cast the
// payment refuses).
func TestRestrictedSourceRefusesModalBackFace(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _, _ := plannedOfferSetup(t, reg, []*cards.Card{card(t, legendCrest), searchCorpusCard(t, reg, "Swamp"),
		searchCorpusCard(t, reg, "Kolvori, God of Kinship")}, "Legend Crest", "Swamp")
	kolvori := searchMoveByName(t, e, "Kolvori, God of Kinship", state.ZHand)
	d := ppAsk(t, e)
	for _, o := range d.Options {
		if o.Obj == kolvori && o.Mode == "modal_spell" {
			t.Fatalf("offered %q: the restricted green cannot pay a legendary artifact spell", o.Label)
		}
	}
	if ppHasAction(d, kolvori) {
		t.Fatalf("a payment action for Kolvori's back face")
	}
	// the potential planner (the untapped-sources witness) agrees: no plan
	// pays the back face with the restricted green
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Obj == kolvori && pp.Action.Mode == "modal_spell" && pp.Reason == "" {
			t.Fatalf("potential plan for %q: %+v", pp.Action.Label, pp)
		}
	}
}
