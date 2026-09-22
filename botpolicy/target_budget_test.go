package botpolicy

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// A KTarget decision carrying a cumulative MaxTotalTargetPower$ budget
// (Reunion of the House: "any number of creature cards with total power 10
// or less") must be answered by the deterministic bot with a set whose
// Value sum fits the budget. The targeting heuristic is price-blind -- it
// fires its full legal width (Min 0 picks up to Max) -- so without the
// Clamp repair its answer would bust Decision.MaxSum, Decision.Validate
// would reject it before consuming the decision, and the bot would
// re-derive the same rejected answer forever. Clamp's FitRequired repair is
// the ONE home of the budget rule for this kind, so this test runs the
// bot's own answer through Validate on the board where the constraint
// binds.
func TestTargetBudgetClampKeepsTheBotAnswerValid(t *testing.T) {
	b := Board{Cards: map[state.ObjID]Card{}}
	d := decision.Decision{
		Player: 0,
		Kind:   decision.KTarget,
		Min:    0,
		Max:    4,
		MaxSum: 10,
		Options: []decision.Option{
			{Index: 0, Kind: "permanent", Obj: state.ObjID(1), Value: 11}, // over the cap alone
			{Index: 1, Kind: "permanent", Obj: state.ObjID(2), Value: 6},
			{Index: 2, Kind: "permanent", Obj: state.ObjID(3), Value: 4},
			{Index: 3, Kind: "permanent", Obj: state.ObjID(4), Value: 3},
		},
	}
	in := Decide(b, &d, rng(1))
	if err := d.Validate(in); err != nil {
		t.Fatalf("bot answer %v failed Validate: %v", in.Choices, err)
	}
	sum := 0
	for _, c := range in.Choices {
		sum += d.Options[c].Value
	}
	if sum > 10 {
		t.Fatalf("choices %v sum to %d, exceeding the cap 10", in.Choices, sum)
	}
	// The 11-power option can never be part of a legal answer.
	for _, c := range in.Choices {
		if c == 0 {
			t.Fatalf("the bot picked the individually over-budget option 0")
		}
	}
}

// A budget-less KTarget decision keeps the plain full-width answer
// byte-for-byte -- Option.Value is unset on every existing target ask, so
// no bot-answered game changes.
func TestTargetWithoutMaxSumUnchanged(t *testing.T) {
	b := Board{Cards: map[state.ObjID]Card{}}
	d := decision.Decision{
		Player: 0,
		Kind:   decision.KTarget,
		Min:    0,
		Max:    3,
		Options: []decision.Option{
			{Index: 0, Kind: "permanent", Obj: state.ObjID(1)},
			{Index: 1, Kind: "permanent", Obj: state.ObjID(2)},
			{Index: 2, Kind: "permanent", Obj: state.ObjID(3)},
		},
	}
	in := Decide(b, &d, rng(1))
	if !reflect.DeepEqual(in.Choices, []int{0, 1, 2}) {
		t.Fatalf("choices = %v, want all three (full-width policy unchanged)", in.Choices)
	}
}
