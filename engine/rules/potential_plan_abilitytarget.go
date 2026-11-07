package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// abilityTargetPricedPlan is planAbilityPaymentLifePips's counterpart of
// potentialTargetPricedPlan: an activated ability a ValidTarget$ cost static
// prices lower for some legal target (Dwarven Mauler: Equip abilities that
// target it cost {2} less, CR 601.2f) gets the witness of the first target
// whose own price the sources can fund, or "insufficient" when none does.
// ok is false when no such static applies, leaving the caller's outcome.
func (e *Engine) abilityTargetPricedPlan(p state.PlayerID, id state.ObjID, ab *cards.SA, cost Cost) (PaymentPlanOutcome, bool) {
	scope := abilityScope(ab)
	statics := e.collectCostStatics()
	if !statics.validTarget {
		return PaymentPlanOutcome{}, false
	}
	if _, max := targetBounds(ab); max > 1 {
		return PaymentPlanOutcome{}, false // a multi-target price is not tried per target
	}
	got := PaymentPlanOutcome{Reason: "insufficient"}
	e.potentialCostModsUsing(statics, p, id, scope, e.costPotentialTargets(p, id, scope), 0, func(m costMods) bool {
		var c Cost
		e.composedOfferCostInto(&c, p, id, &cost, &m, scope)
		got = e.planComposedCost(p, id, c, true, true)
		return got.Plan != nil && got.Reason == ""
	})
	return got, true
}
