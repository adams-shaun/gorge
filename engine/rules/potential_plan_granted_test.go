package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	// ppHolder's printed {2}: Draw shares Ability 0 with whatever it is granted.
	ppHolder = "Name:Potential Holder\nManaCost:1\nTypes:Creature Bear\nPT:2/2\n" +
		"A:AB$ Draw | Cost$ 2 | NumCards$ 1 | SpellDescription$ Draw a card.\nOracle:x\n"
	ppGranter = "Name:Potential Granter\nManaCost:1\nTypes:Artifact Equipment\n" +
		"S:Mode$ Continuous | AffectedDefined$ Equipped | Affected$ Creature | AddAbility$ Life | Description$ x\n" +
		"SVar:Life:AB$ GainLife | Cost$ 1 | LifeAmount$ 1 | SpellDescription$ You gain 1 life.\nOracle:x\n"
	// ppLantern gives the creatures its controller controls an any-colour mana ability.
	ppLantern = "Name:Potential Lantern\nManaCost:1\nTypes:Artifact\n" +
		"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddAbility$ AnyMana | Description$ x\n" +
		"SVar:AnyMana:AB$ Mana | Produced$ Any | Cost$ T | SpellDescription$ x\nOracle:x\n"
)

func ppPrintedHolder(t *testing.T, e *Engine, holder state.ObjID) PotentialPlan {
	t.Helper()
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind == "ability" && pp.Action.Obj == holder && pp.Action.Label == "Potential Holder: Draw a card." {
			return pp
		}
	}
	t.Fatal("the holder's printed ability is not a potential play")
	return PotentialPlan{}
}

// A granted ability's Option has Ability 0 too, and a printed ability at index 0
// stays priced beside it: two Plains witness the holder's {2}.
func TestPotentialPaymentPlansPricesAPrintedAbilityBesideAGrant(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9404, ppFiller)
	holder := onBoard(t, e, 0, ppHolder)
	e.emit(events.Event{Kind: events.Attach, Obj: onBoard(t, e, 0, ppGranter), IDs: []state.ObjID{holder}})
	onBoard(t, e, 0, lrPlains)
	onBoard(t, e, 0, lrPlains)
	ppPriority(t, e)
	if pp := ppPrintedHolder(t, e, holder); pp.Reason != "" || pp.Plan == nil {
		t.Fatalf("printed ability verdict %+v, want a witness", pp)
	}
}

// A prefix search for a printed ability does not stop where the pool first pays a
// granted one on the same permanent: {1} floating offers the granted Life, and the
// printed {2} needs both granted-mana creatures tapped.
func TestPotentialPrefixScriptPassesAGrantedOptionOnTheSameObject(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9404, ppFiller)
	holder := onBoardReady(t, e, 0, ppHolder)
	e.emit(events.Event{Kind: events.Attach, Obj: onBoard(t, e, 0, ppGranter), IDs: []state.ObjID{holder}})
	onBoard(t, e, 0, ppLantern)
	onBoardReady(t, e, 0, ppHolder)
	ppPriority(t, e)
	pp := ppPrintedHolder(t, e, holder)
	if pp.Reason != "" {
		t.Fatalf("printed ability verdict %+v, want a script", pp)
	}
	taps := 0
	for _, st := range pp.Script {
		if st.Kind == decision.KPriority {
			taps++
		}
	}
	if taps != 2 {
		t.Fatalf("the script activates %d sources, want the 2 the {2} needs: %+v", taps, pp.Script)
	}
}
