package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// madnessProvenUnpayable reports whether the planner PROVES p cannot pay the
// madness cast of id (CR 702.35a, 106.1), which withholds the trigger's "yes".
// potentialModeCastPlan's raw "insufficient" is a proof only when the source
// census covers every mana ability p could activate, or a relaxation of the
// abilities it misses still cannot pay: PotentialPaymentPlans' own proof. A
// counter-cost source the census misses (Vivid Creek's "{T}, remove a charge
// counter: one mana of any color") can still make the owed colour, so a
// madness cost only it pays is not proven unpayable.
func madnessProvenUnpayable(e *Engine, p state.PlayerID, id state.ObjID) bool {
	e.beginDerivedMemo()
	defer e.endDerivedMemo()
	defer pay.PaymentPlanQueryEnd(asPayer(e), pay.PaymentPlanQueryBegin(asPayer(e)))
	o := decision.Option{Kind: "cast", Obj: id, Mode: altMode(altMadness)}
	verdict := func() PaymentPlanOutcome { return e.potentialModeCastPlan(p, o) }
	if got := verdict(); got.Plan != nil || got.Reason != pay.ReasonInsufficient {
		return false
	}
	hyp := e.PotentialMana(p)
	census := e.paymentPlanCensusOf(p, &hyp)
	if census.complete {
		return true
	}
	return census.relaxable && e.paymentPlanRelaxProof(census, verdict).Reason == pay.ReasonInsufficient
}
