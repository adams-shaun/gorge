package searchprobe

import (
	"math"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// leafFixture is TestTeacherChoiceRollsEveryCandidateOnSampledWorlds' fixture:
// two sampled worlds at a priority root, every root option as a candidate.
// Measured at the corpus pin: three options; candidates 0 and 1 survive a
// short horizon (non-terminal leaves) and candidate 2 ends the game at once
// (a terminal loss on both worlds).
func leafFixture(t *testing.T) ([]World, [][]Action) {
	t.Helper()
	setup, h := samplingHistory(t)
	result, err := Sample(setup, h, SampleOptions{Seed: 44, Attempts: 8, Worlds: 2, MaxSubmits: 5000})
	if err != nil || len(result.Worlds) != 2 {
		t.Fatalf("sample %d worlds, %v", len(result.Worlds), err)
	}
	d := result.Worlds[0].Engine.Pending()
	var cands [][]Action
	for i := range d.Options {
		a, err := result.Worlds[0].Observer.Actions(d, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{i}})
		if err != nil {
			t.Fatal(err)
		}
		cands = append(cands, a)
	}
	if len(cands) != 3 {
		t.Fatalf("fixture root offers %d options, want 3 (corpus pin moved? re-measure the goldens below)", len(cands))
	}
	return result.Worlds, cands
}

// A nil Leaf is the heuristic leaf, bit for bit. The golden was measured with
// the pre-Leaf TeacherChoice (the parent of the commit that added Leaf) on
// this fixture; a corpus pin move can change it, which the fixture's option
// count guard above usually reports first.
func TestTeacherChoiceNilLeafIsTheHeuristicBitForBit(t *testing.T) {
	worlds, cands := leafFixture(t)
	got, err := TeacherChoice(worlds, cands, TeacherOptions{Seed: 3, HorizonTurns: 2, MaxSubmits: 5000})
	if err != nil {
		t.Fatal(err)
	}
	want := []uint64{0x3fe06660f0a5ea1e, 0x3fe06660f0a5ea1e, 0}
	for i, v := range got.Values {
		if math.Float64bits(v) != want[i] {
			t.Fatalf("value %d = %#x, want %#x (all %v)", i, math.Float64bits(v), want[i], got.Values)
		}
	}
	if got.Index != 0 || got.Rollouts != 6 || got.Submits != 110 || got.Terminal != 2 || got.Capped != 0 {
		t.Fatalf("counts changed: %+v", got)
	}
}

// A Leaf returning a candidate-dependent constant decides the choice exactly.
// With Parallelism 1 the rollouts run in (world, candidate) order and only
// non-terminal leaves call Leaf, so over the two surviving candidates call k
// belongs to candidate k%2.
func TestTeacherChoiceLeafFlipsTheChoice(t *testing.T) {
	worlds, cands := leafFixture(t)
	cands = cands[:2]
	for _, tc := range []struct {
		vals  [2]float64
		index int
	}{{[2]float64{0.25, 0.75}, 1}, {[2]float64{0.75, 0.25}, 0}} {
		var calls int
		leaf := func(view.View, state.PlayerID) float64 {
			x := tc.vals[calls%2]
			calls++
			return x
		}
		got, err := TeacherChoice(worlds, cands, TeacherOptions{Seed: 3, HorizonTurns: 2, MaxSubmits: 5000, Leaf: leaf})
		if err != nil {
			t.Fatal(err)
		}
		if got.Terminal != 0 || calls != got.Rollouts {
			t.Fatalf("want every leaf non-terminal and scored once: %d calls, %+v", calls, got)
		}
		if got.Index != tc.index || got.Values[0] != tc.vals[0] || got.Values[1] != tc.vals[1] {
			t.Fatalf("leaf %v: index %d values %v", tc.vals, got.Index, got.Values)
		}
	}
}

// A terminal leaf keeps 1 / 0 / 0.5 and never calls Leaf.
func TestTeacherChoiceTerminalLeafNeverCallsLeaf(t *testing.T) {
	worlds, cands := leafFixture(t)
	var calls atomic.Int64
	leaf := func(view.View, state.PlayerID) float64 { calls.Add(1); return 0.5 }
	// Game-end rollouts: every leaf is terminal.
	got, err := TeacherChoice(worlds, cands, TeacherOptions{Seed: 3, MaxSubmits: 5000, Leaf: leaf})
	if err != nil {
		t.Fatal(err)
	}
	if got.Terminal != got.Rollouts || calls.Load() != 0 {
		t.Fatalf("game-end rollouts called Leaf %d times: %+v", calls.Load(), got)
	}
	// Mixed: candidate 2 ends the game at once; a Leaf answering 1 must not
	// lift its loss off 0, and is called once per non-terminal rollout.
	got, err = TeacherChoice(worlds, cands, TeacherOptions{Seed: 3, HorizonTurns: 2, MaxSubmits: 5000,
		Leaf: func(view.View, state.PlayerID) float64 { calls.Add(1); return 1 }})
	if err != nil {
		t.Fatal(err)
	}
	if got.Terminal != 2 || got.Values[2] != 0 || got.Values[0] != 1 || calls.Load() != int64(got.Rollouts-got.Terminal) {
		t.Fatalf("terminal leaf scored by Leaf: %d calls, %+v", calls.Load(), got)
	}
	// And directly: an Over view never reaches the evaluator.
	w := state.PlayerID(1)
	boom := func(view.View, state.PlayerID) float64 { t.Fatal("Leaf called on a terminal view"); return 0 }
	if leafValue(boom, view.View{Over: true, Winner: &w}, 0) != 0 || leafValue(boom, view.View{Over: true, Draw: true}, 0) != 0.5 {
		t.Fatal("terminal values")
	}
}

// Leaf results are clamped into [0,1]; NaN is the no-information 0.5.
func TestLeafValueClampsTheEvaluator(t *testing.T) {
	for _, c := range []struct{ in, want float64 }{{-3, 0}, {0, 0}, {0.3, 0.3}, {1, 1}, {7, 1}, {math.NaN(), 0.5}, {math.Inf(1), 1}, {math.Inf(-1), 0}} {
		if got := leafValue(func(view.View, state.PlayerID) float64 { return c.in }, view.View{}, 0); got != c.want {
			t.Fatalf("leaf %v -> %v, want %v", c.in, got, c.want)
		}
	}
}

// Parallelism never changes the result, with a (pure) Leaf set too.
func TestTeacherChoiceLeafIndependentOfParallelism(t *testing.T) {
	worlds, cands := leafFixture(t)
	leaf := func(v view.View, actor state.PlayerID) float64 {
		return 1 / (1 + math.Exp(-LeafScore(v, actor)/5))
	}
	opts := TeacherOptions{Seed: 3, HorizonTurns: 2, MaxSubmits: 5000, Leaf: leaf}
	seq, err := TeacherChoice(worlds, cands, opts)
	if err != nil {
		t.Fatal(err)
	}
	opts.Parallelism = 4
	par, err := TeacherChoice(worlds, cands, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seq, par) {
		t.Fatalf("parallelism changed the result:\n%+v\n%+v", seq, par)
	}
	for i := range seq.Values {
		if math.Float64bits(seq.Values[i]) != math.Float64bits(par.Values[i]) {
			t.Fatalf("value %d bits differ", i)
		}
	}
}
