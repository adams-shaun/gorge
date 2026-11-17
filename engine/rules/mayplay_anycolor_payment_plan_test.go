package rules

// The payment-window planner under a may-play rider must fund a relaxed
// coloured pip from a coloured unit. "Any color" never includes colourless
// (CR 107.4c), so the folded witness that prices the pip generic and taps a
// colourless ability cannot settle the real payment: the cast reverts with the
// card still in the graveyard and the pool holding stranded {C}. The kernel
// taps the coloured ability and casts the card.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// mayPlayAnyColorCoastBoard is seat 0's board for a may-play cast under
// MayPlayIgnoreColor$ whose only untapped source is a land with both a
// colourless and a coloured mana ability: the grant + creature in the
// graveyard, an empty pool, and a fresh priority ask.
func mayPlayAnyColorCoastBoard(t *testing.T, manaCost string) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e, creature := mayPlayRiderBoard(t, "MayPlayIgnoreColor$ True | ", manaCost, 0)
	coast := onBoard(t, e, 0, "Name:Test Coast\nTypes:Land\n"+
		"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Colourless.\n"+
		"A:AB$ Mana | Cost$ T | Produced$ Combo G U | SpellDescription$ Green or blue.\nOracle:x\n")
	e.pending = nil
	e.askPriority(0)
	if o := e.G.Obj(creature); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("creature in zone %v, want the graveyard", o.Zone)
	}
	if e.G.Players[0].Pool != (state.Mana{}) {
		t.Fatalf("pool = %v, want empty", e.G.Players[0].Pool)
	}
	return e, creature, coast
}

// colouredActivation reports whether plan activates source's coloured ability.
func colouredActivation(plan decision.PaymentPlan, source state.ObjID) bool {
	for _, a := range plan.Activations {
		if a.Source != source {
			continue
		}
		for _, c := range []int{state.MW, state.MU, state.MB, state.MR, state.MG} {
			if a.Produces[c] > 0 {
				return true
			}
		}
	}
	return false
}

// TestMayPlayAnyColorPaymentPlanFundsTheColouredPip: a {R} may-play cast from
// an empty pool beside one dual-ability land must get a one-click plan that
// activates the coloured Combo ability, and Submit(Payment) must stack the
// creature, tap the land and empty the pool.
func TestMayPlayAnyColorPaymentPlanFundsTheColouredPip(t *testing.T) {
	t.Parallel()
	e, creature, coast := mayPlayAnyColorCoastBoard(t, "R")
	a, ok := mayPlayAction(e, creature)
	if !ok {
		t.Fatalf("no payment action for %d under the any-color rider", creature)
	}
	if len(a.Plans) != 1 {
		t.Fatalf("payment action offers %d plans, want one", len(a.Plans))
	}
	if !colouredActivation(a.Plans[0], coast) {
		t.Fatalf("plan does not activate the coloured ability: %+v", a.Plans[0])
	}
	submitPaymentPlan(t, e, e.Pending(), a)
	if got := e.G.Obj(creature).Zone; got != state.ZStack {
		t.Fatalf("creature in zone %v, want the stack", got)
	}
	if o := e.G.Obj(coast); o == nil || !o.Tapped {
		t.Fatalf("the source is not tapped after the planned payment: %+v", o)
	}
	if pool := e.G.Players[0].Pool; pool != (state.Mana{}) {
		t.Fatalf("pool = %v after the planned payment, want empty", pool)
	}
}

// TestMayPlayAnyColorPaymentPlanAnswerValidates runs the plan the auto-paying
// seat answers with through Decision.Validate, then Engine.ValidateCastPayment
// (the witness settle), then Submit, so the plan the wire accepts is exactly
// the plan the executor settles.
func TestMayPlayAnyColorPaymentPlanAnswerValidates(t *testing.T) {
	t.Parallel()
	e, creature, coast := mayPlayAnyColorCoastBoard(t, "R")
	a, ok := mayPlayAction(e, creature)
	if !ok {
		t.Fatalf("no payment action for %d under the any-color rider", creature)
	}
	d := e.Pending()
	sel := &decision.PaymentSelection{ActionID: a.ID, Plan: decision.ClonePaymentPlan(a.Plans[0])}
	in := decision.Intent{Seq: d.Seq, Player: d.Player, Payment: sel}
	if !colouredActivation(sel.Plan, coast) {
		t.Fatalf("the answered plan does not activate the coloured ability: %+v", sel.Plan)
	}
	if err := d.Validate(in); err != nil {
		t.Fatalf("Decision.Validate: %v", err)
	}
	if err := e.ValidateCastPayment(0, a.Cast, sel.Plan); err != nil {
		t.Fatalf("ValidateCastPayment: %v", err)
	}
	if err := e.Submit(in); err != nil {
		t.Fatalf("Submit planned payment: %v", err)
	}
	if got := e.G.Obj(creature).Zone; got != state.ZStack {
		t.Fatalf("creature in zone %v, want the stack", got)
	}
	if o := e.G.Obj(coast); o == nil || !o.Tapped {
		t.Fatalf("the source is not tapped after the validated payment: %+v", o)
	}
	if pool := e.G.Players[0].Pool; pool != (state.Mana{}) {
		t.Fatalf("pool = %v after the validated payment, want empty", pool)
	}
}
