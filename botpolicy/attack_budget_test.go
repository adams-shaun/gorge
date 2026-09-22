package botpolicy

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

// TestClampTrimsOverBudgetAttackers: a KAttackers decision now carries a
// cumulative attack-cost budget (Decision.MaxSum over each pair's Option.Value
// -- the CantAttackUnless prop static, rules/attack_cost.go). The combat
// heuristic (chooseAttackersMode) is price-blind, so for a budgeted
// declaration it can return a set whose Value sum exceeds MaxSum, which
// Decision.Validate rejects -- Engine.Submit then rejects before consuming
// the pending decision and the deterministic bot re-derives the same invalid
// answer forever. Clamp must trim the dearest chosen pair until the sum fits.
func TestClampTrimsOverBudgetAttackers(t *testing.T) {
	d := decision.Decision{
		Kind:   decision.KAttackers,
		Min:    0,
		Max:    3,
		MaxSum: 3,
		Options: []decision.Option{
			{Index: 0, Kind: "attacker", Obj: 1, Value: 2},
			{Index: 1, Kind: "attacker", Obj: 2, Value: 2},
			{Index: 2, Kind: "attacker", Obj: 3, Value: 1},
		},
	}
	// The heuristic picked all three (sum 5 > 3): Clamp drops the dearest
	// (index 0 or 1, tie broken by higher index, so index 1) and then fits.
	in := Clamp(&d, decision.Intent{Choices: []int{0, 1, 2}})
	if err := d.Validate(in); err != nil {
		t.Fatalf("clamped answer %v failed Validate: %v", in.Choices, err)
	}
	sum := 0
	for _, c := range in.Choices {
		sum += d.Options[c].Value
	}
	if sum > d.MaxSum {
		t.Fatalf("clamped sum %d exceeds MaxSum %d (choices %v)", sum, d.MaxSum, in.Choices)
	}
}

// TestClampTrimKeepsRequiredAttackers: a Required pair (CR 508.1d) is dropped
// only after every non-Required pair is gone, so a declaration the engine's
// requirement solver demands is not silently trimmed away.
func TestClampTrimKeepsRequiredAttackers(t *testing.T) {
	d := decision.Decision{
		Kind:   decision.KAttackers,
		Min:    0,
		Max:    3,
		MaxSum: 3,
		Options: []decision.Option{
			{Index: 0, Kind: "attacker", Obj: 1, Value: 3, Required: true},
			{Index: 1, Kind: "attacker", Obj: 2, Value: 2},
			{Index: 2, Kind: "attacker", Obj: 3, Value: 2},
		},
	}
	in := Clamp(&d, decision.Intent{Choices: []int{0, 1, 2}})
	if err := d.Validate(in); err != nil {
		t.Fatalf("clamped answer %v failed Validate: %v", in.Choices, err)
	}
	if !reflect.DeepEqual(in.Choices, []int{0}) {
		t.Fatalf("choices = %v, want [0] (required kept, both unrequired pairs dropped)", in.Choices)
	}
}

// TestClampLeavesBudgetFreeAttackersUntouched: with no prop in play the
// decision has no MaxSum and Clamp must not reorder or drop any pair -- every
// ordinary declaration is byte-identical to before the prop work.
func TestClampLeavesBudgetFreeAttackersUntouched(t *testing.T) {
	d := decision.Decision{
		Kind: decision.KAttackers,
		Min:  0,
		Max:  3,
		Options: []decision.Option{
			{Index: 0, Kind: "attacker", Obj: 1},
			{Index: 1, Kind: "attacker", Obj: 2},
			{Index: 2, Kind: "attacker", Obj: 3},
		},
	}
	in := Clamp(&d, decision.Intent{Choices: []int{0, 1, 2}})
	if !reflect.DeepEqual(in.Choices, []int{0, 1, 2}) {
		t.Fatalf("choices = %v, want all three unchanged", in.Choices)
	}
}
