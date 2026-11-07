package rules

import (
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// targetPricedAbilityPlan is an activation's witness when a ValidTarget$ cost
// static prices it lower for some legal target (CR 601.2f reads the chosen
// targets: Cloud, Planet's Champion's "Equip abilities you activate that
// target Cloud cost {2} less"). The printed composition never sees a target,
// so planAbilityPaymentLifePips' first plan charges the full cost; this
// retries one target assignment at a time (potentialCostModsUsing), as
// potentialTargetPricedPlan does for a cast and offerCastableUsing's
// potential pass does for the offer gate, and the target ask keeps the
// targets the floated mana pays for. ok is false when no ValidTarget$ static
// is in play or the ability has no single target, so the caller keeps its answer.
func targetPricedAbilityPlan(e *Engine, p state.PlayerID, id state.ObjID, cost Cost, scope costScope) (PaymentPlanOutcome, bool) {
	statics := e.collectCostStatics()
	if !statics.validTarget {
		return PaymentPlanOutcome{}, false
	}
	targets := e.costPotentialTargets(p, id, scope)
	if len(targets) == 0 {
		return PaymentPlanOutcome{}, false
	}
	if sa := pay.CostTargetingSA(e.G, id, scope); sa != nil {
		if _, max := targetBounds(sa); max > 1 {
			// The target ask tests each candidate as the sole selection,
			// which does not price an ability choosing several.
			return PaymentPlanOutcome{}, false
		}
	}
	got := PaymentPlanOutcome{Reason: "insufficient"}
	e.potentialCostModsUsing(statics, p, id, scope, targets, 0, func(m costMods) bool {
		var c Cost
		e.composedOfferCostInto(&c, p, id, &cost, &m, scope)
		got = e.planComposedCost(p, id, c, true, true)
		return got.Plan != nil && got.Reason == ""
	})
	return got, true
}
