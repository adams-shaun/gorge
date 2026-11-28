package decision

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestPaymentPlanRejectsMoreThanOneVictim pins the validator's victim bound:
// the V1 other-sacrifice shape names exactly one permanent, so a plan with a
// longer list is malformed before the duplicate scan walks it (a crafted
// intent cannot make validation quadratic).
func TestPaymentPlanRejectsMoreThanOneVictim(t *testing.T) {
	_, in := paymentFixture(t)
	cast := PlannedCast{Object: 17, Face: 0, Origin: "hand"}
	plan := ClonePaymentPlan(in.Payment.Plan)
	plan.Activations[0].Consequence = &PaymentConsequence{SacrificeOthers: true}

	plan.Activations[0].Sacrifices = []state.ObjID{7}
	if _, err := PaymentPlanID(123, 0, cast, plan); err != nil {
		t.Fatalf("one victim: the closed shape must validate: %v", err)
	}

	plan.Activations[0].Sacrifices = []state.ObjID{7, 8}
	if _, err := PaymentPlanID(123, 0, cast, plan); err == nil || !strings.Contains(err.Error(), "maximum") {
		t.Fatalf("two victims: err = %v, want the victim bound", err)
	}

	huge := make([]state.ObjID, 4096)
	for i := range huge {
		huge[i] = state.ObjID(i + 1)
	}
	plan.Activations[0].Sacrifices = huge
	if _, err := PaymentPlanID(123, 0, cast, plan); err == nil || !strings.Contains(err.Error(), "maximum") {
		t.Fatalf("4096 victims: err = %v, want the victim bound", err)
	}
}
