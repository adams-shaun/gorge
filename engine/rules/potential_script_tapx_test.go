package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// tapXBoard puts board on seat 0's battlefield (in that order) at its first
// main phase of turn 3 and returns the engine.
func tapXBoard(t *testing.T, board []string) *Engine {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var deck []*cards.Card
	for _, n := range board {
		deck = append(deck, lookup(t, reg, n))
	}
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{append(deck, mountainDeck(t, 40-len(deck))...), mountainDeck(t, 40)},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	for _, n := range board {
		moveToBattlefield(t, e, findAndMoveToHand(t, e, 0, n))
	}
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	return e
}

func tapXPlay(t *testing.T, e *Engine, name string) PotentialPlan {
	t.Helper()
	for _, pp := range e.PotentialPaymentPlans(0) {
		if o := e.G.Obj(pp.Action.Obj); pp.Action.Kind == "ability" && o != nil && o.Face().Name == name {
			return pp
		}
	}
	t.Fatalf("%s has no ability among the potential plays", name)
	return PotentialPlan{}
}

// TestPrefixScriptDoesNotTapTheAbilitysSourceForAnothersCost: Nutrient Block's
// "{2}, {T}, sacrifice" is payable with a Forest and Gene Pollinator's mana
// ("{T}, tap an untapped permanent you control"), but the permanent that
// Pollinator taps may not be the Block, whose own {T} is then unpayable. The
// Block is first in battlefield order; the script must tap the Bears.
func TestPrefixScriptDoesNotTapTheAbilitysSourceForAnothersCost(t *testing.T) {
	t.Parallel()
	e := tapXBoard(t, []string{"Nutrient Block", "Gene Pollinator", "Forest", "Grizzly Bears"})
	block := tapXPlay(t, e, "Nutrient Block")
	if block.Reason != "" || len(block.Script) == 0 {
		t.Fatalf("Nutrient Block: reason %q detail %q, %d script steps, want a script", block.Reason, block.Detail, len(block.Script))
	}
	for _, st := range block.Script {
		for _, pk := range st.Picks {
			if pk.Kind == "tapcost" && e.G.Obj(pk.Obj).Face().Name == "Nutrient Block" {
				t.Fatalf("the script taps the Block for Pollinator's cost: %+v", block.Script)
			}
		}
	}
}

// TestPrefixScriptPassesForATriggerBeforeASorceryPlay: Bonesplitter's
// "Equip {1}" (sorcery speed) is payable with Gene Pollinator's mana alone. Tapping Hemosymbic Mite for Pollinator's cost stacks the
// Mite's trigger, and the equip is not offered until it resolves, so the
// script ends by passing priority once.
func TestPrefixScriptPassesForATriggerBeforeASorceryPlay(t *testing.T) {
	t.Parallel()
	e := tapXBoard(t, []string{"Hemosymbic Mite", "Bonesplitter", "Gene Pollinator"})
	var gear state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if e.G.Obj(id).Face().Name == "Bonesplitter" {
			gear = id
		}
	}
	var equip PotentialPlan
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind == "ability" && pp.Action.Obj == gear {
			equip = pp
		}
	}
	if equip.Reason != "" || len(equip.Script) == 0 {
		t.Fatalf("Equip: reason %q detail %q, %d script steps, want a script", equip.Reason, equip.Detail, len(equip.Script))
	}
	last := equip.Script[len(equip.Script)-1]
	if last.Kind != decision.KPriority || len(last.Picks) != 1 || last.Picks[0].Kind != "pass" {
		t.Fatalf("the script's last step is %+v, want a pass for the Mite's trigger: %+v", last, equip.Script)
	}
}
