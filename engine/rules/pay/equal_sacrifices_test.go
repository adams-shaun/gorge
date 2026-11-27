package pay

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Two plans that differ only in the victims a step sacrifices are different
// plans: PlanOutcomeEqual must keep reflect.DeepEqual's answer on the
// Sacrifices lists, nil versus empty included.
func TestPayEqualComparesSacrifices(t *testing.T) {
	lists := [][]state.ObjID{nil, {}, {4}, {5}, {4, 5}, {5, 4}}
	mk := func(v []state.ObjID) PlanOutcome {
		cons := &decision.PaymentConsequence{SacrificeOthers: true}
		return PlanOutcome{Plan: &decision.PaymentPlan{Version: 1, ID: "a", Activations: []decision.PaymentActivation{{Source: 3, Consequence: cons, Sacrifices: v}}}}
	}
	distinct := 0
	for i := range lists {
		for j := range lists {
			a, b := mk(lists[i]), mk(lists[j])
			want := reflect.DeepEqual(a, b)
			if i != j && !want {
				distinct++
			}
			if got := PlanOutcomeEqual(a, b); got != want {
				t.Errorf("PlanOutcomeEqual(%v, %v) = %v, DeepEqual %v", lists[i], lists[j], got, want)
			}
		}
	}
	if distinct == 0 {
		t.Fatal("precondition: no pair of victim lists differs")
	}
}
