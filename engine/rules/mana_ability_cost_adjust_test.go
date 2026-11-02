package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A mana ability is an activated ability, so a cost static that reaches it
// prices it: CR 605.1a, 602.2b and 601.2f; Forge runs CostAdjustment.adjust on
// a mana ability's cost like any other's.

const paidTapLandFixtureSrc = `Name:Fixture Paid Tap Land
ManaCost:no cost
Types:Land
A:AB$ Mana | Cost$ 1 T | Produced$ G | SpellDescription$ Add {G}.
Oracle:{1}, {T}: Add {G}.
`

const landDiscountFixtureSrc = `Name:Fixture Land Discount
ManaCost:1 G
Types:Creature Beast
PT:1/1
S:Mode$ ReduceCost | Type$ Ability | ValidCard$ Land.YouCtrl | Amount$ 1
Oracle:Activated abilities of lands you control cost {1} less to activate.
`

const landSurchargeFixtureSrc = `Name:Fixture Land Surcharge
ManaCost:1 G
Types:Creature Beast
PT:1/1
S:Mode$ RaiseCost | Type$ Ability | ValidCard$ Land.YouCtrl | Amount$ 1
Oracle:Activated abilities of lands you control cost {1} more to activate.
`

const landSurchargeSparedFixtureSrc = `Name:Fixture Land Surcharge Spared
ManaCost:1 G
Types:Creature Beast
PT:1/1
S:Mode$ RaiseCost | ValidCard$ Land.YouCtrl | ValidSpell$ Activated.!ManaAbility | Amount$ 1
Oracle:Activated abilities of lands you control cost {1} more to activate unless they're mana abilities.
`

func manaAbilityOffered(e *Engine, id state.ObjID) bool {
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return false
	}
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Obj == id {
			return true
		}
	}
	return false
}

// TestReducedManaAbilityCostIsPaid: with the discount on the battlefield the
// paid ability costs only its {T}, so it is offered with an empty pool and the
// activation spends nothing from a floating mana.
func TestReducedManaAbilityCostIsPaid(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 6301, paidTapLandFixtureSrc, landDiscountFixtureSrc)
	land := putCreature(t, e, 0, paidTapLandFixtureSrc)
	e.priorityRound()
	if manaAbilityOffered(e, land) {
		t.Fatalf("precondition: the {1},{T} land is offered with an empty pool and no discount")
	}
	putCreature(t, e, 0, landDiscountFixtureSrc)
	e.priorityRound()
	if !manaAbilityOffered(e, land) {
		t.Fatalf("a land discount leaves {T}; the mana ability must be offered with an empty pool")
	}
	activateMana(t, e, land)
	if got := e.G.Players[0].Pool[state.MG]; got != 1 {
		t.Fatalf("pool G = %d, want 1 (the reduced ability costs no mana)", got)
	}
}

// TestReducedFilterLandKeepsFloatingMana: a floating {G} stays when the
// reduction made the filter's activation free.
func TestReducedFilterLandKeepsFloatingMana(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 6302, paidTapLandFixtureSrc, landDiscountFixtureSrc)
	land := putCreature(t, e, 0, paidTapLandFixtureSrc)
	putCreature(t, e, 0, landDiscountFixtureSrc)
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "W", Amount: 1})
	e.priorityRound()
	activateMana(t, e, land)
	pool := e.G.Players[0].Pool
	if pool[state.MW] != 1 || pool[state.MG] != 1 {
		t.Fatalf("pool = %v, want the floating W kept and the land's G added", pool)
	}
}

// TestRaisedManaAbilityCostIsPaid: a surcharge on a land's activated
// abilities lands on a bare-{T} mana ability too, so it needs the mana.
func TestRaisedManaAbilityCostIsPaid(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 6303, forestFixtureSrc, landSurchargeFixtureSrc)
	land := putCreature(t, e, 0, forestFixtureSrc)
	e.priorityRound()
	if !manaAbilityOffered(e, land) {
		t.Fatalf("precondition: a plain land is offered with an empty pool")
	}
	putCreature(t, e, 0, landSurchargeFixtureSrc)
	e.priorityRound()
	if manaAbilityOffered(e, land) {
		t.Fatalf("a {1} surcharge makes the land's mana ability cost {1},{T}; it is not payable with an empty pool")
	}
}

// TestSurchargeThatSparesManaAbilitiesLeavesThem: ValidSpell$
// Activated.!ManaAbility keeps the static off mana abilities.
func TestSurchargeThatSparesManaAbilitiesLeavesThem(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 6304, forestFixtureSrc, landSurchargeSparedFixtureSrc)
	land := putCreature(t, e, 0, forestFixtureSrc)
	putCreature(t, e, 0, landSurchargeSparedFixtureSrc)
	e.priorityRound()
	if !manaAbilityOffered(e, land) {
		t.Fatalf("a surcharge that spares mana abilities must not price the land's mana ability")
	}
}

const filterLandFixtureSrc = `Name:Fixture Filter Land
ManaCost:no cost
Types:Land
A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add {C}.
A:AB$ Mana | Cost$ 1 T | Produced$ Any | SpellDescription$ Add one mana of any color.
Oracle:{T}: Add {C}.\n{1}, {T}: Add one mana of any color.
`

const islandFixtureSrc = "Name:Fixture Island\nManaCost:no cost\nTypes:Basic Land Island\nOracle:x\n"

const duoFixtureSrc = `Name:Fixture Duo
ManaCost:G U
Types:Instant
A:SP$ Draw | NumCards$ 1
Oracle:x
`

// TestReducedFilterLandPaysTheCast: {G}{U} from an Island and a filter land
// needs the filter to make the G. Priced at its printed {1} it has to be fed
// the Island's mana (one mana in all); the discount makes it a free
// any-colour source, so the payment planner proves the cast and its plan taps
// the filter land for G.
func TestReducedFilterLandPaysTheCast(t *testing.T) {
	t.Parallel()
	for _, discount := range []bool{false, true} {
		e, _, duo := newFixtureDeck(t, 6305, duoFixtureSrc)
		onBoard(t, e, 0, filterLandFixtureSrc)
		onBoard(t, e, 0, islandFixtureSrc)
		if discount {
			onBoard(t, e, 0, landDiscountFixtureSrc)
		}
		ppPriority(t, e)
		var pp PotentialPlan
		for _, cand := range e.PotentialPaymentPlans(0) {
			if cand.Action.Kind == "cast" && cand.Action.Obj == duo {
				pp = cand
			}
		}
		if discount && pp.Reason != "" || !discount && pp.Reason == "" && pp.Action.Obj == duo {
			t.Fatalf("discount=%v: verdict %+v, want a plan exactly when the filter land is free", discount, pp)
		}
		if discount && pp.Action.Obj != duo {
			t.Fatalf("with the land discount the {G}{U} cast has a payment plan; got none")
		}
	}
}
