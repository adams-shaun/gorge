package rules

import (
	"github.com/adams-shaun/gorge/engine/effects"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
)

// cost_paid_x_bound.go is the paid-X cost filter: a Sac/Discard/Exile part
// whose spec compares against the {X} the cost itself announces --
// Knollspine Invocation's "{X}, Discard a card with mana value X"
// (Cost$ X Discard<1/Card.cmcEQX>, SVar:X:Count$xPaid).
//
// CR 601.2b announces X before CR 601.2h pays costs (CR 602.2b for an
// activated ability), so once the pending payment has settled X the part is
// matched with X bound to it (payer.MatchesSpecFrom). A bare spec resolve evaluates Count$xPaid in a
// context with no announced X and reads 0, which offered only the mana value
// 0 cards.

// costPaidXVarName is the variable name a paid-X cost spec is read for.
const costPaidXVarName = "X"

// paidXSVarBody is the SVar body that binds X to the paid {X}.
const paidXSVarBody = "Count$xPaid"

// costSpecPaidXBound reports whether spec compares against an X whose SVar
// body (the face's SVar:X) binds it to the paid {X}. Pure over its inputs:
// the caller holds the pending payment and the engine.
func costSpecPaidXBound(spec, xBody string) bool {
	return costvocab.SpecComparesVar(spec, costPaidXVarName) &&
		trimParam(xBody) == paidXSVarBody
}

// withPaidX is sc with X resolving to the announced x, every other name
// resolving as before.
func withPaidX(sc effects.SpecContext, x int32) effects.SpecContext {
	prev := sc.Resolve
	sc.Resolve = func(name string) (int32, bool) {
		if name == costPaidXVarName {
			return x, true
		}
		return prev(name)
	}
	return sc
}
