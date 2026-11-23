package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Gorge's side of dirac's Feed the Cycle boards (dirac/act_forage_offer_test.go
// TestFeedTheCycleForageStepExilesTheFodder / ...LeavesEnoughFodder, lockstep
// pair std-blb 184): Thornvault Forager's {T}, Forage: Add two mana beside a
// Prophetic Prism ({1},{T}: Add one mana of any color, a filter that nets no
// mana). The forage's exile of three graveyard cards leaves the cast's own
// forage arm (the either-or additional cost) unpayable, so the cast costs
// {1}{B}{B} against two mana (CR 601.2g-h, 701.61a). These tests pin gorge's
// answer, the one the kernel's forage step must match; they pass with or
// without the dirac fix.

// feedTheCycleForageBoard seeds Thornvault Forager and a Prophetic Prism
// (untapped, not summoning-sick) on seat 0's battlefield, the named cards in
// its graveyard and Feed the Cycle in hand, and returns the engine parked at a
// priority decision with the cast's object.
func feedTheCycleForageBoard(t *testing.T, grave []string) (*Engine, state.ObjID) {
	t.Helper()
	hero := append([]string{"Feed the Cycle", "Thornvault Forager", "Prophetic Prism"}, grave...)
	e, _, _ := altCostEngine(t, 933, hero, nil, []string{altBearSrc})
	ftc := findCardObj(t, e, 0, "Feed the Cycle", state.ZHand)
	for _, name := range []string{"Thornvault Forager", "Prophetic Prism"} {
		id := findCardObj(t, e, 0, name, state.ZBattlefield)
		e.G.Obj(id).SummonSick = false
	}
	for _, name := range grave {
		findCardObj(t, e, 0, name, state.ZGraveyard)
	}
	if e.G.Obj(findCardObj(t, e, 1, "Bear", state.ZBattlefield)).Zone != state.ZBattlefield {
		t.Fatal("no opposing creature: the destroy target is a precondition")
	}
	e.pending = nil
	e.priorityRound()
	return e, ftc
}

// feedTheCyclePlan is the payment planner's verdict on the cast: the lockstep
// oracle takes gorge's offer of a play from PotentialPaymentPlans (dirac/oracle
// gorge.go priorityVocabulary), since the interactive walk offers a cast only
// from the floating pool. Reason "" is a payable play, "insufficient" a proof
// it is not, "unsupported" a shape it does not price (no offer either).
func feedTheCyclePlan(t *testing.T, e *Engine, ftc state.ObjID) PotentialPlan {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("pending %v, want a priority decision", d)
	}
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind == "cast" && pp.Action.Obj == ftc {
			return pp
		}
	}
	t.Fatal("the planner prices no cast of Feed the Cycle")
	return PotentialPlan{}
}

// TestAltAddForageOfferExilesTheForageFodder: three graveyard cards, and
// Thornvault's forage exiles them -- two mana (the Prism nets none) for
// {1}{B}{B}: the planner prices it unpayable (it reads
// "unsupported", census_incomplete, for the forage source), so the oracle
// takes no offer of it.
func TestAltAddForageOfferExilesTheForageFodder(t *testing.T) {
	e, ftc := feedTheCycleForageBoard(t, []string{"Bark-Knuckle Boxer", "Sunshower Druid", "Heaped Harvest"})
	if pp := feedTheCyclePlan(t, e, ftc); pp.Reason == "" {
		t.Fatalf("Feed the Cycle priced payable (plan %+v, script %v): Thornvault's forage exiles the cast's forage fodder, leaving two mana for {1}{B}{B}", pp.Plan, pp.Script)
	}
}

// TestAltAddForageOfferLeavesEnoughFodder: six graveyard cards pay both
// forages, and the two mana fund {1}{B}: offered.
func TestAltAddForageOfferLeavesEnoughFodder(t *testing.T) {
	e, ftc := feedTheCycleForageBoard(t, []string{"Bark-Knuckle Boxer", "Sunshower Druid", "Heaped Harvest", "Grizzly Bears", "Hill Giant", "Craw Wurm"})
	if pp := feedTheCyclePlan(t, e, ftc); pp.Reason != "" {
		t.Fatalf("Feed the Cycle priced %q (%s): six graveyard cards pay both forages and two mana fund {1}{B}", pp.Reason, pp.Detail)
	}
}
