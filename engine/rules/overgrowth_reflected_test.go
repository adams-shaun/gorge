package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const overgrowthProbe = "Name:Triple Probe\nManaCost:1 G G\nTypes:Instant\nA:SP$ Draw | Num$ 1\nOracle:x\n"

// CR 605.1b, 106.12a: Overgrowth on a Reflecting Pool makes one tap yield {G}{G}{G}
// (the pool's reflected {G} plus the trigger's {G}{G}), so a {1}{G}{G} spell is
// payable from the Pool alone: the bound counts the trigger's mana on a reflected
// production and the cast is offered (decision 206 of dr-mbc-aaaa pair 10).
func TestOvergrowthOnReflectingPoolFundsTripleMana(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9711, overgrowthProbe)
	forest := onBoard(t, e, 0, reflForest)
	e.G.Obj(forest).Tapped = true
	pool := onBoardCard(t, e, 0, corpusCardOrFatal(t, "Reflecting Pool"))
	aura := onBoardCard(t, e, 0, corpusCardOrFatal(t, "Overgrowth"))
	e.emit(events.Event{Kind: events.Attach, Obj: aura, IDs: []state.ObjID{pool}})
	if o := e.G.Obj(aura); o.AttachedTo != pool {
		t.Fatalf("precondition: Overgrowth attached to %d, want the Pool %d", o.AttachedTo, pool)
	}
	if b := e.PotentialMana(0); b[state.ManaIndex('G')] < 3 {
		t.Errorf("PotentialMana G = %d, want >= 3", b[state.ManaIndex('G')])
	}
	driveToStep(t, e, 1, 0, state.StepMain1)
	priced := false
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind == "cast" && pp.Action.Obj == spell {
			priced = pp.Reason == "" && pp.Plan != nil
		}
	}
	if !priced {
		t.Errorf("{1}{G}{G} spell has no priced potential plan with an untapped Pool carrying Overgrowth")
	}
}
