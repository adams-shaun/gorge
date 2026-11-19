package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTwobridPlannedFromUntappedSources holds the planner to CR 107.4e and
// 601.2f: a {2/W} pip is payable with either one white mana or two generic, so
// the planner must resolve each pip's face and plan the resolved cost instead
// of declining the whole cost shape. The colour-face case is the load-bearing
// one: three Plains cannot fund six generic, so only a plan that pays one
// white per pip passes; the generic case pins the other face.
func TestTwobridPlannedFromUntappedSources(t *testing.T) {
	for _, c := range []struct {
		name  string
		board []string
		hand  []string
	}{
		{"every pip as its colour", []string{"Plains", "Plains", "Plains"}, []string{"Spectral Procession"}},
		{"every pip as two generic", []string{"Mountain", "Mountain", "Mountain", "Mountain", "Mountain", "Mountain"}, []string{"Spectral Procession"}},
		{"mixed faces", []string{"Plains", "Plains", "Mountain", "Mountain"}, []string{"Spectral Procession"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := boundBoard(t, c.board, c.hand)
			id := boundHandID(t, e, "Spectral Procession")
			tapped := 0
			for _, oid := range e.G.Zone(state.ZBattlefield, 0) {
				if ob := e.G.Obj(oid); ob != nil && ob.Tapped {
					tapped++
				}
			}
			if tapped != 0 {
				t.Fatalf("precondition: %d board sources are tapped, want every source available", tapped)
			}
			plans := e.PotentialPaymentPlans(0)
			var plan *PotentialPlan
			for i := range plans {
				if plans[i].Action.Obj == id {
					plan = &plans[i]
					break
				}
			}
			if plan == nil {
				t.Fatalf("Spectral Procession is not among the potential plays")
			}
			if plan.Reason != "" {
				t.Fatalf("Spectral Procession plan reason %q detail %q, want a payable plan", plan.Reason, plan.Detail)
			}
			if plan.Plan == nil || len(plan.Plan.Activations) == 0 {
				t.Fatalf("Spectral Procession has no witness: %+v", plan)
			}
		})
	}
}

// TestTwobridCastPlanIsOffered holds the priority offer: a {2/W} cast the
// untapped sources can pay must carry a V1 witness (PlanCastPayment) and be
// listed at priority.
func TestTwobridCastPlanIsOffered(t *testing.T) {
	e := boundBoard(t, []string{"Plains", "Plains", "Plains"}, []string{"Spectral Procession"})
	id := boundHandID(t, e, "Spectral Procession")
	out := e.PlanCastPayment(0, decision.PlannedCast{Object: id, Origin: "hand"})
	if out.Reason != "" || out.Plan == nil {
		t.Fatalf("PlanCastPayment = reason %q detail %q plan %v; shape=%q candidate=%v, want a witness", out.Reason, out.Detail, out.Plan, e.paymentPlanCastShapeDetail(0, id), e.paymentPlanCastCandidate(0, id))
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("precondition: pending decision is %v, want priority", d)
	}
	for _, a := range e.PaymentActionsForPriority(0, d.Seq) {
		if a.Cast.Object == id {
			return
		}
	}
	t.Fatalf("the payment actions do not offer the twobrid cast")
}

// TestTwobridUnpayableCastIsNotOffered pins the other direction: Karn's five
// {2/X} pips cost at least seven mana from W2 G2 B1 (the three missing
// colours must use their two-generic faces), so no witness exists and the
// planner must not invent one.
func TestTwobridUnpayableCastIsNotOffered(t *testing.T) {
	e := boundBoard(t, []string{"Plains", "Forest", "Plains", "Forest", "Swamp"}, []string{"Karn, Gilded Guardian"})
	id := boundHandID(t, e, "Karn, Gilded Guardian")
	out := e.PlanCastPayment(0, decision.PlannedCast{Object: id, Origin: "hand"})
	if out.Plan != nil || out.Reason != "insufficient" {
		t.Fatalf("PlanCastPayment = reason %q detail %q plan %v, want insufficient", out.Reason, out.Detail, out.Plan)
	}
	d := e.Pending()
	for _, a := range e.PaymentActionsForPriority(0, d.Seq) {
		if a.Cast.Object == id {
			t.Fatalf("the payment actions offer an unpayable twobrid cast")
		}
	}
}
