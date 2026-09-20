package botpolicy

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// TestDigBudgetArmFillsGreedilyUnderMaxSum: a Dig carrying a cumulative
// WithTotalCMC$ budget (Decision.MaxSum) must be answered with a set whose
// Value sum fits the budget -- a blind first-Max answer fails Validate and
// the engine rejects it, so the bot would livelock re-deriving it. The arm
// fills greedily in offered order: [2,3,2] under budget 4 takes the two
// 2-MV cards (3 does not fit after the first 2) and passes Validate.
func TestDigBudgetArmFillsGreedilyUnderMaxSum(t *testing.T) {
	b := Board{Cards: map[state.ObjID]Card{}}
	d := decision.Decision{
		Kind:   decision.KChoose,
		Min:    0,
		Max:    3,
		MaxSum: 4,
		Options: []decision.Option{
			{Index: 0, Kind: "dig", Obj: 1, Value: 2},
			{Index: 1, Kind: "dig", Obj: 2, Value: 3},
			{Index: 2, Kind: "dig", Obj: 3, Value: 2},
		},
	}
	in := Decide(b, &d, rng(1))
	if err := d.Validate(in); err != nil {
		t.Fatalf("bot answer %v failed Validate: %v", in.Choices, err)
	}
	if !reflect.DeepEqual(in.Choices, []int{0, 2}) {
		t.Fatalf("choices = %v, want [0 2] (greedy fill: 2 fits, 3 does not, 2 fits)", in.Choices)
	}
}

// TestDigBudgetArmUnchangedWithoutMaxSum: a budget-less Dig keeps the plain
// first-Max policy byte-for-byte, so every existing bot-answered game is
// unaffected by this change.
func TestDigBudgetArmUnchangedWithoutMaxSum(t *testing.T) {
	b := Board{Cards: map[state.ObjID]Card{}}
	d := decision.Decision{
		Kind: decision.KChoose,
		Min:  0,
		Max:  3,
		Options: []decision.Option{
			{Index: 0, Kind: "dig", Obj: 1},
			{Index: 1, Kind: "dig", Obj: 2},
			{Index: 2, Kind: "dig", Obj: 3},
		},
	}
	in := Decide(b, &d, rng(1))
	if !reflect.DeepEqual(in.Choices, []int{0, 1, 2}) {
		t.Fatalf("choices = %v, want all three (first-Max policy unchanged)", in.Choices)
	}
}
