package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// planImprovise plans the mana a plain Improvise cast still owes after the
// artifacts in taps pay generic mana (CR 702.126a): mana is the credited
// price (improviseCost over the printed cost, then the cost statics), and
// the artifacts are withheld from the plan's sources because one permanent
// pays one cost (CR 601.2g-h). The coloured pips an artifact cannot pay are
// paid by the lands' mana abilities, so a cast the floating pool alone cannot
// pay still gets a witness (CR 601.2h).
//
// The credit is greedy (the offer's), so an "insufficient" answer is not a
// proof: a mana artifact the credit tapped might have paid a pip a different
// announcement leaves it free for. That outcome is declined as an unknown
// credit shape instead of reported as unpayable.
func planImprovise(pe pay.Engine, p state.PlayerID, cast decision.PlannedCast, mana Cost, desc *pay.Descriptor, taps []state.ObjID) PaymentPlanOutcome {
	got := pay.PlanPaymentCostWithout(pe, p, cast, mana, desc, taps, nil, nil)
	if got.Plan == nil && got.Reason == "insufficient" {
		return PaymentPlanOutcome{Reason: "unsupported", Detail: "cost:credit"}
	}
	return got
}
