package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestUnlessCostPayableFeeLeavesManaLeft pins the fee transition of the
// unless-cost reach walk (engine/rules/pay/planshape.go feeFromMana): a mana
// ability's literal fee is charged from the floating mana BEFORE the
// activation resolves (CR 605.3b), so the mana it consumes is GONE -- what
// funds the rest of the walk is the pool the fee left, not the pool spent.
// The walk must therefore price every later payable() against the mana left,
// or it double-counts the fee's mana as still floating and offers an unless
// payment the window cannot complete.
func TestUnlessCostPayableFeeLeavesManaLeft(t *testing.T) {
	t.Parallel()
	isle := "Name:Fee Isle\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ U\nOracle:x\n"
	wizard := "Name:Apprentice Wizard\nManaCost:1 U U\nTypes:Creature Human Wizard\nPT:0/1\n" +
		"A:AB$ Mana | Cost$ U T | Produced$ C | Amount$ 3 | SpellDescription$ Add {C}{C}{C}.\nOracle:x\n"
	// placeWizard boards the wizard off the sick (it has been in play since
	// last turn; CR 302.6 bars a sick creature's tap ability).
	placeWizard := func(e *Engine) state.ObjID {
		id := onBoard(t, e, 0, wizard)
		e.G.Obj(id).SummonSick = false
		return id
	}

	// Precondition: the census prices the wizard as a fee-carrying source, so
	// the negative below really is the fee transition's answer and not a
	// source the census never listed.
	ePre := handEngine(t)
	ePre.G.Players[0].Pool = state.Mana{}
	islePre := onBoard(t, ePre, 0, isle)
	wizPre := placeWizard(ePre)
	for _, id := range []state.ObjID{islePre, wizPre} {
		if o := ePre.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
			t.Fatalf("precondition: source %d not untapped on the battlefield: %+v", id, o)
		}
	}
	feeListed := false
	for _, u := range ePre.unlessWindowUnits(0) {
		for _, a := range u.Alts {
			if a.CostGeneric+a.CostColored.Total() > 0 {
				feeListed = true
			}
		}
	}
	if !feeListed {
		t.Fatal("precondition: the unless census listed no fee-carrying source — premise false")
	}

	// Island + wizard from an empty pool: tapping the Isle floats {U}, the
	// wizard's {U} fee consumes it, and the wizard's production {C}{C}{C} is
	// all that is left -- a {4} tax is NOT payable. Priced against the mana
	// spent instead of the mana left, the walk would still count the spent
	// {U} and offer a Pay the window cannot complete.
	e := handEngine(t)
	e.G.Players[0].Pool = state.Mana{}
	onBoard(t, e, 0, isle)
	placeWizard(e)
	if e.UnlessCostPayable(0, "4") {
		t.Fatal("the Island's {U} dies into the wizard's {U} fee (CR 605.3b); the chain floats only {C}{C}{C}, so a {4} tax is not payable")
	}
	if !e.UnlessCostPayable(0, "3") {
		t.Fatal("the chain's settled {C}{C}{C} must make a {3} tax payable — the fee arm stopped promising the chain")
	}

	// Two Islands + wizard: the first Isle's {U} funds the fee, the second
	// Isle's {U} survives it, and {U}{C}{C}{C} reaches a {4} tax. This is the
	// chain the fee arm exists to price.
	e2 := handEngine(t)
	e2.G.Players[0].Pool = state.Mana{}
	onBoard(t, e2, 0, isle)
	onBoard(t, e2, 0, isle)
	placeWizard(e2)
	if !e2.UnlessCostPayable(0, "4") {
		t.Fatal("the two-Islands + wizard chain reaches {U}{C}{C}{C}; a {4} tax is payable")
	}
}
