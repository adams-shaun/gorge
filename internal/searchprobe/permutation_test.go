package searchprobe

import (
	"math"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/state"
)

type fixedRandom struct {
	values []int
	at     int
}

func (r *fixedRandom) IntN(n int) int {
	if r.at >= len(r.values) {
		panic("unexpected random draw")
	}
	v := r.values[r.at]
	r.at++
	if v < 0 || v >= n {
		panic("test random value out of range")
	}
	return v
}

// These probabilities are hand-counted over six equally likely physical-card
// permutations. Equal names must not collapse distinct physical copies.
func TestPermutationExactThreeCardDistribution(t *testing.T) {
	cards := []state.ObjID{11, 22, 33}
	for _, tc := range []struct {
		name   string
		fixed  []placement
		paths  [][]int
		want   [][]state.ObjID
		weight float64
	}{
		{"unconstrained", nil, [][]int{{0, 0}, {0, 1}, {1, 0}, {1, 1}, {2, 0}, {2, 1}}, [][]state.ObjID{{22, 33, 11}, {33, 22, 11}, {33, 11, 22}, {11, 33, 22}, {22, 11, 33}, {11, 22, 33}}, 1},
		{"one fixed", []placement{{0, 22}}, [][]int{{0}, {1}}, [][]state.ObjID{{22, 33, 11}, {22, 11, 33}}, 1.0 / 3},
		{"two fixed", []placement{{0, 22}, {2, 11}}, [][]int{{}}, [][]state.ObjID{{22, 33, 11}}, 1.0 / 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i, path := range tc.paths {
				r := &fixedRandom{values: path}
				got, logWeight, err := samplePermutation(cards, tc.fixed, r)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, tc.want[i]) {
					t.Fatalf("path %v: got %v want %v", path, got, tc.want[i])
				}
				if math.Abs(math.Exp(logWeight)-tc.weight) > 1e-14 {
					t.Fatalf("weight=%g want %g", math.Exp(logWeight), tc.weight)
				}
				// q = 1/number of allowed permutations; corrected mass is 1/6.
				if math.Abs(math.Exp(logWeight)/float64(len(tc.paths))-1.0/6) > 1e-14 {
					t.Fatal("proposal weight does not recover the uniform target")
				}
				if r.at != len(path) {
					t.Fatal("wrong number of random draws")
				}
			}
		})
	}
	if !reflect.DeepEqual(cards, []state.ObjID{11, 22, 33}) {
		t.Fatal("mutated source card pool")
	}
}

func TestPermutationRejectsContradictoryAssignments(t *testing.T) {
	for _, fixed := range [][]placement{
		{{-1, 11}}, {{3, 11}}, {{0, 99}}, {{0, 11}, {0, 22}}, {{0, 11}, {1, 11}},
	} {
		if _, _, err := samplePermutation([]state.ObjID{11, 22, 33}, fixed, &fixedRandom{}); err == nil {
			t.Fatalf("accepted %+v", fixed)
		}
	}
	if _, _, err := samplePermutation([]state.ObjID{11, 11}, nil, &fixedRandom{}); err == nil {
		t.Fatal("accepted duplicate physical copies")
	}
}

func TestPermutationTapeReproducesEveryThreeCardOrder(t *testing.T) {
	before := []state.ObjID{11, 22, 33}
	for _, after := range [][]state.ObjID{{11, 22, 33}, {11, 33, 22}, {22, 11, 33}, {22, 33, 11}, {33, 11, 22}, {33, 22, 11}} {
		tape, err := permutationTape(before, after)
		if err != nil {
			t.Fatal(err)
		}
		if len(tape) != 2 {
			t.Fatalf("draw count=%d", len(tape))
		}
		got := append([]state.ObjID(nil), before...)
		for j, d := range tape {
			i := 2 - j
			if d.Bound != i+1 || d.Value < 0 || d.Value > d.Bound-1 {
				t.Fatalf("invalid draw %+v", d)
			}
			got[i], got[d.Value] = got[d.Value], got[i]
		}
		if !reflect.DeepEqual(got, after) {
			t.Fatalf("got %v want %v", got, after)
		}
	}
	for _, after := range [][]state.ObjID{{11, 22}, {11, 11, 33}, {11, 22, 99}} {
		if _, err := permutationTape(before, after); err == nil {
			t.Fatalf("accepted non-permutation %v", after)
		}
	}
}
