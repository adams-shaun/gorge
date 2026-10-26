package rules

// respay.go is the pay-first model the resolution-time mana payment windows
// share: a triggered ability's optional Cost$ (e.triggerCost), cumulative
// upkeep (e.cumulative) and echo (e.echo). Each of them used to open a
// manual "Tap <source> for mana / Done" window BEFORE its pay/decline (or
// pay/sacrifice) election, so a Frenzied Goblin attack posed a mana window
// and then "Pay {R} / Do not pay", and the priority-time auto-pay plans
// (payment_plan.go) never reached it.
//
// Now the election comes first. Its pay option is offered when the pool
// alone pays, or when it does not but an untapped source could help (the
// condition that used to open the window). Answering pay arms resPay:
//
//   - the AUTO route (the plain pay option) runs the payment planner's
//     witness one activation at a time, re-planning from the live state
//     after each step, until the pool covers the cost; a cost the planner
//     declines, or a step it cannot run, falls back to the manual window;
//   - the MANUAL route (the "<flow>_pay_manual" option, offered beside the
//     plain one only when the planner has a witness) opens the familiar
//     manual window, whose Done settles the payment.
//
// Either route ends in the flow's own charge: a pool that still cannot pay
// is the decline (the sacrifice, for cumulative upkeep and echo), exactly as
// an unpaid election always was. Every tap is an ordinary mana-ability
// resolution through events, re-derived on replay from the same planner, so
// nothing here is nondeterministic or bypasses events.Apply.

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// resPayAutoStepCap bounds the auto route's activations for one payment: a
// step always taps or spends a source, so no real cost needs more than the
// payer's sources, and the cap keeps a misbehaving source from looping.
const resPayAutoStepCap = 64

// resPay is one window's pay-first state. A value struct, so the window
// clones that copy their holder carry it.
type resPay struct {
	// chosen marks the payer's pay election: the window now funds the cost
	// and then charges it, with no second election.
	chosen bool
	// manual routes the funding through the manual window (the payer's
	// choice, or the auto route's fallback).
	manual bool
	// steps counts the auto route's activations (resPayAutoStepCap).
	steps int
	// entries counts the funding re-entries, so an auto step whose mana
	// ability did not continue the window (it produced nothing) is noticed
	// and the window re-entered by hand.
	entries int
}

// resPayNeedsMana reports whether a pay election for amount must still fund
// it from the payer's sources: the cost is one the manual window can help
// with (Priceable), the pool alone does not pay it -- with the payer's
// PayLifeInsteadOf grant suspended, exactly as the window's own gate does --
// an untapped source that yields mana exists, and the seat's potential mana
// (the pool plus every source's upper bound, counting only the mana
// abilities whose RestrictValid$ admits this payment) can cover it. False means the
// election is settled by the pool alone, as it always was: a cost no source
// can reach keeps that path's decline-only ask, never a pay that can only
// float mana and then fail at the charge.
func (e *Engine) resPayNeedsMana(player state.PlayerID, source state.ObjID, amount Cost, class paymentClass) bool {
	if !amount.Priceable() {
		return false
	}
	rider := asEval(e).MayPlayRider(player, source)
	desc := paymentDescriptor{ID: source, Class: class, Cost: &amount}
	if pay.CostPayableClassLife(asPayer(e), player, desc, rider, amount, false) {
		return false
	}
	sources := false
	for _, id := range e.G.Zone(state.ZBattlefield, player) {
		if e.untappedManaSource(player, id) && e.windowSourceYields(player, id) {
			sources = true
			break
		}
	}
	return sources && pay.CostPayablePool(asPayer(e), player, source, false, amount,
		e.potentialManaFor(player, &desc), e.G.Players[player].ManaUnits())
}

// resPayPlan is the planner's witness for amount's mana from the current
// state, or nil when the planner cannot fund it (an unsupported cost shape,
// an unplannable pool, or sources that do not cover it). Life is charged by
// the flow's own payment after the mana is in the pool, so it is not part of
// the planned shape.
func (e *Engine) resPayPlan(player state.PlayerID, source state.ObjID, amount Cost, class paymentClass) *decision.PaymentPlan {
	planned := amount
	planned.Life = 0
	if pay.PlanCostDetail(planned) != "" || !pay.PlanPoolOK(&e.G.Players[player]) {
		return nil
	}
	desc := paymentDescriptor{ID: source, Class: class, Cost: &amount}
	got := pay.PlanPaymentCost(asPayer(e), player, decision.PlannedCast{Object: source, Origin: "resolution"}, planned, &desc)
	if got.Plan == nil || len(got.Plan.Activations) == 0 {
		return nil
	}
	return got.Plan
}

// resPayOptions returns the election's pay option(s) for a cost that must
// still be funded: the plain pay (payKind), and -- only when the planner can
// fund it, so the plain pay is the auto route -- the manual route
// (manualKind) after it. With no witness the plain pay IS the manual route,
// so it is offered alone.
func (e *Engine) resPayOptions(player state.PlayerID, source state.ObjID, amount Cost, class paymentClass, payKind, manualKind, label string) []decision.Option {
	opts := []decision.Option{{Kind: payKind, Obj: source, Label: label}}
	if e.resPayPlan(player, source, amount, class) != nil {
		opts = append(opts, decision.Option{Kind: manualKind, Obj: source, Label: label + " (choose mana sources)"})
	}
	return opts
}

// resPayElect records the pay election: manual picks the manual route.
func (rp *resPay) elect(manual bool) {
	rp.chosen, rp.manual = true, manual
}

// resPayFund advances an elected payment's funding. It returns true when it
// has posed a decision or activated a source (whose continuation re-enters
// the window through paymentWindowAsk), false once the pool pays the cost or
// nothing more can be funded -- the caller then charges.
func (e *Engine) resPayFund(rp *resPay, player state.PlayerID, source state.ObjID, amount Cost,
	windowDone bool, prompt string, flow chooseFor, class paymentClass) bool {
	rp.entries++
	if !e.resPayNeedsMana(player, source, amount, class) {
		return false
	}
	if !rp.manual {
		if rp.steps < resPayAutoStepCap {
			if plan := e.resPayPlan(player, source, amount, class); plan != nil && e.resPayAutoStep(rp, player, plan.Activations[0]) {
				return true
			}
		}
		rp.manual = true
	}
	return e.paymentManaAskClass(player, source, amount, windowDone, prompt, flow, class)
}

// resPayAutoStep runs one planned activation through the same resolution the
// manual window's tap makes (a payment-window activation, whose continuation
// re-enters the window). It reports false, having changed nothing, when the
// step no longer matches the live sources.
func (e *Engine) resPayAutoStep(rp *resPay, player state.PlayerID, pa decision.PaymentActivation) bool {
	step, reason := pay.PaymentPlanStepReady(asPayer(e), player, e.paymentPlanManaUnits(player), pa)
	if reason != "" {
		return false
	}
	rp.steps++
	entries, mark := rp.entries, len(e.L.Events)
	ma := step.Ma
	e.resolveManaAbilityRefOriginal(player, pa.Source, pay.AlternativeExec(step), ma,
		pay.GainedManaRefFor(asPayer(e), player, pa.Source, ma), false, true, true)
	if rp.entries == entries && e.pending == nil && e.choosing == chooseNone {
		// The activation neither continued the window nor posed a decision
		// (it could not be paid after all): stop the auto route and re-enter
		// on the manual one, so the payment can never wedge.
		if len(e.L.Events) == mark {
			rp.manual = true
		}
		e.paymentWindowAsk()
	}
	return true
}
