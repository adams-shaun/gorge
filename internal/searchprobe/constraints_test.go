package searchprobe

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/state"
)

type rankRandom struct{ value uint64 }

func (r *rankRandom) Uint64() uint64 { return r.value }

func TestConstrainedPermutationExactDistribution(t *testing.T) {
	cards := []proposalCard{{1, "A"}, {2, "A"}, {3, "B"}, {4, "C"}}
	tests := []struct {
		name      string
		positions []positionConstraint
		deadlines []deadlineConstraint
		count     int
	}{
		{name: "unconstrained", count: 24},
		{name: "fixed name", positions: []positionConstraint{{Index: 0, Name: "A"}}, count: 12},
		{name: "fixed object", positions: []positionConstraint{{Index: 0, Obj: 1}}, count: 6},
		{name: "deadline", deadlines: []deadlineConstraint{{Through: 1, Name: "A", Count: 1}}, count: 12},
		{name: "nested deadlines", deadlines: []deadlineConstraint{{Through: 1, Name: "A", Count: 1}, {Through: 3, Name: "B", Count: 1}}, count: 8},
		{name: "object and deadline", positions: []positionConstraint{{Index: 0, Obj: 1}}, deadlines: []deadlineConstraint{{Through: 2, Name: "B", Count: 1}}, count: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			seen := make(map[string]bool)
			for rank := 0; rank < tc.count; rank++ {
				order, logWeight, compatible, err := sampleConstrainedPermutation(cards, tc.positions, tc.deadlines, &rankRandom{value: uint64(rank)})
				if err != nil {
					t.Fatal(err)
				}
				if !compatible {
					t.Fatal("compatible constraints rejected")
				}
				if math.Abs(math.Exp(logWeight)-float64(tc.count)/24) > 1e-12 {
					t.Fatalf("weight=%g want %g", math.Exp(logWeight), float64(tc.count)/24)
				}
				if !permutationSatisfies(order, cards, tc.positions, tc.deadlines) {
					t.Fatalf("order %v violates constraints", order)
				}
				seen[keyIDs(order)] = true
			}
			if len(seen) != tc.count {
				t.Fatalf("sampled %d unique orders, want %d", len(seen), tc.count)
			}
		})
	}
}

func TestConstrainedPermutationRejectsInvalidOrImpossibleConstraints(t *testing.T) {
	cards := []proposalCard{{1, "A"}, {2, "B"}, {3, "C"}}
	invalid := []struct {
		positions []positionConstraint
		deadlines []deadlineConstraint
	}{
		{positions: []positionConstraint{{Index: -1, Name: "A"}}},
		{positions: []positionConstraint{{Index: 3, Name: "A"}}},
		{positions: []positionConstraint{{Index: 0, Obj: 99}}},
		{positions: []positionConstraint{{Index: 0, Obj: 1}, {Index: 1, Obj: 1}}},
		{positions: []positionConstraint{{Index: 0, Name: "A"}, {Index: 0, Name: "B"}}},
		{deadlines: []deadlineConstraint{{Through: 4, Name: "A", Count: 1}}},
		{deadlines: []deadlineConstraint{{Through: 1, Name: "A", Count: -1}}},
	}
	for _, tc := range invalid {
		if _, _, _, err := sampleConstrainedPermutation(cards, tc.positions, tc.deadlines, &rankRandom{}); err == nil {
			t.Fatalf("accepted invalid positions=%v deadlines=%v", tc.positions, tc.deadlines)
		}
	}
	_, _, compatible, err := sampleConstrainedPermutation(cards, nil, []deadlineConstraint{{Through: 1, Name: "A", Count: 2}}, &rankRandom{})
	if err != nil || compatible {
		t.Fatalf("impossible constraints: compatible=%v err=%v", compatible, err)
	}
}

func permutationSatisfies(order []state.ObjID, cards []proposalCard, positions []positionConstraint, deadlines []deadlineConstraint) bool {
	names := make(map[state.ObjID]string)
	for _, card := range cards {
		names[card.ID] = card.Name
	}
	for _, c := range positions {
		if c.Obj != 0 && order[c.Index] != c.Obj || c.Name != "" && names[order[c.Index]] != c.Name {
			return false
		}
	}
	for _, c := range deadlines {
		count := 0
		for _, id := range order[:c.Through] {
			if names[id] == c.Name {
				count++
			}
		}
		if count < c.Count {
			return false
		}
	}
	return true
}

func keyIDs(ids []state.ObjID) string {
	return fmt.Sprint(ids)
}

func TestConstrainedPermutationDoesNotMutateInput(t *testing.T) {
	cards := []proposalCard{{1, "A"}, {2, "A"}, {3, "B"}}
	want := append([]proposalCard(nil), cards...)
	_, _, _, err := sampleConstrainedPermutation(cards, []positionConstraint{{Index: 0, Name: "A"}}, nil, &rankRandom{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cards, want) {
		t.Fatalf("mutated cards: %v", cards)
	}
}
