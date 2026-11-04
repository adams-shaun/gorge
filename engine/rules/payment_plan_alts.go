package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
)

// MaxPaymentPlans is how many plans EnsurePaymentPlanAlternatives offers per
// planned cast: the planner's own plan and up to two whose residuals differ.
const MaxPaymentPlans = 3

// EnsurePaymentPlanAlternatives builds the current priority decision's
// payment extension (EnsurePaymentActions) and gives each planned cast up to
// MaxPaymentPlans plans whose residuals differ: the multiset of sources,
// by the planner's class of interchangeable units, left untapped, and the
// consequences paid (pay.ResidualSignature). Mountain, Mountain, Mountain,
// Swamp, Swamp pay {B}{R} one way and {1}{B} two (Swamp and Mountain, or two
// Swamps). Plans[0] stays the planner's own choice, so a reader of plan 0
// sees exactly the ordinary extension; the alternatives follow it in rank
// order. Only a plain mana cost is given alternatives: a cast whose cost
// also taps or sacrifices a permanent keeps its one plan.
//
// It is a pure derived read like EnsurePaymentActions, built once per
// decision and only when asked: the ordinary offer, the wire and the
// default bot never see the alternatives. Submit builds them when an intent
// names a plan the ordinary extension lacks, so a searched alternative
// validates exactly as plan 0 does.
func (e *Engine) EnsurePaymentPlanAlternatives() []decision.PaymentAction {
	actions := e.EnsurePaymentActions()
	d := e.pending
	if d == nil || d.Kind != decision.KPriority || d.PaymentPlanAlternativesBuilt {
		return actions
	}
	d.PaymentPlanAlternativesBuilt = true
	if len(d.PaymentActions) == 0 {
		return d.PaymentActions
	}
	p := d.Acting()
	e.beginDerivedMemo()
	defer e.endDerivedMemo()
	defer pay.PaymentPlanQueryEnd(asPayer(e), pay.PaymentPlanQueryBegin(asPayer(e)))
	e.PlanAltsWant = MaxPaymentPlans
	defer func() { e.PlanAltsWant, e.PlanAlts = 0, nil }()
	statics := costStaticSource{e: e}
	legal := paymentCastCandidates{e: e, p: p, priced: true}
	for i := range d.PaymentActions {
		a := &d.PaymentActions[i]
		if len(a.Plans) != 1 {
			continue
		}
		e.PlanAlts = nil
		got := e.planCastPaymentChecked(p, a.Cast, &statics, &legal)
		if got.Plan == nil || !samePlanWitness(*got.Plan, a.Plans[0]) {
			// The alternatives belong to plan 0's own search; a plan that is
			// not reproduced gets none.
			continue
		}
		for _, alt := range e.PlanAlts {
			if !e.paymentPlanHoldsOnStack(p, a.Cast.Object, alt) {
				continue
			}
			pid, err := decision.PaymentPlanID(d.Seq, d.Player, a.Cast, alt)
			if err != nil {
				continue
			}
			alt.ID = pid
			a.Plans = append(a.Plans[:len(a.Plans):len(a.Plans)], alt)
		}
	}
	return d.PaymentActions
}

// samePlanWitness reports whether two plans are the same witness, IDs aside.
func samePlanWitness(a, b decision.PaymentPlan) bool {
	a.ID, b.ID = "", ""
	return a.Equal(&b)
}

// paymentPlanOffered reports whether sel names a plan d's extension offers
// (decision.Decision.Validate's own membership test).
func paymentPlanOffered(d *decision.Decision, sel *decision.PaymentSelection) bool {
	for i := range d.PaymentActions {
		a := &d.PaymentActions[i]
		if a.ID != sel.ActionID {
			continue
		}
		for j := range a.Plans {
			if a.Plans[j].Equal(&sel.Plan) {
				return true
			}
		}
		return false
	}
	return false
}
