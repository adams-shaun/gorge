package rules

import "github.com/adams-shaun/gorge/engine/state"

// abilityTargetPricedPlan is a witness for an activated ability whose
// ValidTarget$ (or target-reading) cost statics price it lower for some legal
// target (CR 601.2c, 601.2f; Cloud, Planet's Champion's "Equip abilities that
// target Cloud cost {2} less"). The chosen target is not known when the offer
// is made, so each potential target's price is tried in turn
// (potentialCostModsUsing, the cast path's potentialTargetPricedPlan twin) and
// the first payable plan wins. ok is false when no static reads targets or no
// target's price has a plan; the caller keeps its untargeted outcome.
func (e *Engine) abilityTargetPricedPlan(p state.PlayerID, id state.ObjID, cost Cost, scope costScope) (PaymentPlanOutcome, bool) {
	statics := e.collectCostStatics()
	if !statics.validTarget {
		return PaymentPlanOutcome{}, false
	}
	var got PaymentPlanOutcome
	_, ok := e.potentialCostModsUsing(statics, p, id, scope, e.costPotentialTargets(p, id, scope), 0, func(m costMods) bool {
		var c Cost
		e.composedOfferCostInto(&c, p, id, &cost, &m, scope)
		got = e.planComposedCost(p, id, c, true, true)
		return got.Plan != nil && got.Reason == ""
	})
	return got, ok
}
