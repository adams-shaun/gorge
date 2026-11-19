package gonet

import (
	"encoding/json"
	"math"
	"os"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/engine/decision"
)

// podInputs is the pod set's encoding of gonettest.PodView from seat 0:
// the state and two options (cast the command-zone commander, pass).
func podInputs(t *testing.T) (policynet.State, []policynet.Option) {
	t.Helper()
	v := gonettest.PodView()
	st := policynet.EncodeStateWith(policynet.FeaturesPod, v, 0, nil)
	opts := policynet.EncodeOptionsWith(policynet.FeaturesPod, &v, 0, decision.KPriority,
		[]decision.Option{{Kind: "cast", Obj: 100}, {Kind: "pass"}}, nil)
	// the inputs carry the pod facts the networks read
	if got := st.Dense[policynet.DenseWidth+3*policynet.PodSeatWidth+policynet.PodMonarch]; got != 1 {
		t.Fatalf("pod state: seat 3's monarch column %v", got)
	}
	if got := st.Cards[0].Raw[policynet.PodEntAttacked0+3]; got != 1 {
		t.Fatalf("pod state: the attacker's attacked-seat-3 column %v", got)
	}
	if opts[0].EntA == 0 {
		t.Fatal("pod option: the command-zone cast names no entity")
	}
	return st, opts
}

func finite(xs ...float32) bool {
	for _, x := range xs {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return false
		}
	}
	return true
}

// TestPodMLPForward loads a pod mlp network (seven pooled groups, the pod
// raw and dense widths) and runs it on the four-seat Commander state: the
// forward runs, its outputs are finite, and the monarch column moves them.
// A pod network without the pod encoder_hash is refused.
func TestPodMLPForward(t *testing.T) {
	dir := t.TempDir()
	path, err := gonettest.WritePod(dir, "pod.dzgw")
	if err != nil {
		t.Fatal(err)
	}
	g, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if g.groups != policynet.EntityGroupsPod || g.RawWidth() != policynet.EntityRawWidthPod || !g.EncoderHashChecked() {
		t.Fatalf("pod mlp: groups %d raw %d hash checked %v", g.groups, g.RawWidth(), g.EncoderHashChecked())
	}
	st, opts := podInputs(t)
	scores := make([]float32, len(opts))
	v := g.Eval(&st, opts, scores)
	if !finite(append(scores, v)...) {
		t.Fatalf("pod mlp: value %v scores %v", v, scores)
	}
	st2 := st
	st2.Dense = slices.Clone(st.Dense)
	st2.Dense[policynet.DenseWidth+3*policynet.PodSeatWidth+policynet.PodMonarch] = 0
	if v2 := g.Eval(&st2, opts, make([]float32, len(opts))); v2 == v {
		t.Errorf("pod mlp: the monarch column does not move the value (%v)", v)
	}
	if _, err := gonettest.WriteRaw(dir, "nohash.dzgw", policynet.EntityRawWidthPod, map[string]any{"GROUPS": policynet.EntityGroupsPod, "DENSE_W": policynet.DenseWidthPod}); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir + "/nohash.dzgw"); err == nil {
		t.Error("a pod-group network without the pod encoder hash loaded")
	}
}

// podGolden is the pinned output of WritePodGNN's network on podInputs.
type podGolden struct {
	Value  float32   `json:"value"`
	Scores []float32 `json:"scores"`
	ValueK []float32 `json:"value_k"`
}

// TestGNNPodFourSeat is the graph network's four-seat Commander case: a pod
// graph (player_nodes 4) has four player nodes, one per relative seat, and
// 23 fixed tokens (root, players, each seat's battlefield, graveyard, exile
// and command zone, the hand, the stack); it runs on the pod state through
// the SIMD and the scalar kernels to the same outputs, its batched forward
// equals the single one, the monarch column and the attacker's attacked-seat
// column move it, and its value, option and K-way value logits equal the
// golden (testdata/gnn/pod_golden.json; GORGE_UPDATE_PODGOLDEN=1 rewrites
// it). The golden pins this build's forward, not PyTorch's: dzg's GraphNet
// has no pod graph yet.
func TestGNNPodFourSeat(t *testing.T) {
	path, err := gonettest.WritePodGNN(t.TempDir(), "pod.dzgw", 5)
	if err != nil {
		t.Fatal(err)
	}
	g, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if g.gnn == nil || g.gnn.layout.players != 4 || g.gnn.layout.fixed() != 23 {
		t.Fatalf("pod graph: %+v", g.gnn)
	}
	st, opts := podInputs(t)
	run := func(st *policynet.State) podGolden {
		var out podGolden
		out.Scores = make([]float32, len(opts))
		out.Value = g.Eval(st, opts, out.Scores)
		out.ValueK = make([]float32, g.ValueK())
		g.EvalValueVec(st, out.ValueK)
		return out
	}
	got := run(&st)
	if !finite(append(append(got.Scores, got.Value), got.ValueK...)...) {
		t.Fatalf("pod graph outputs %+v", got)
	}
	ForceScalar = true
	sc := run(&st)
	ForceScalar = false
	near := func(a, b float32) bool { return math.Abs(float64(a-b)) <= 1e-4 }
	if !near(sc.Value, got.Value) {
		t.Errorf("scalar value %v, simd %v", sc.Value, got.Value)
	}
	values := make([]float32, 2)
	bscores := [][]float32{make([]float32, len(opts)), nil}
	g.EvalBatch([]*policynet.State{&st, &st}, [][]policynet.Option{opts, nil}, bscores, values)
	if values[0] != got.Value || values[1] != got.Value || !slices.Equal(bscores[0], got.Scores) {
		t.Errorf("batch %v %v, single %v %v", values, bscores[0], got.Value, got.Scores)
	}
	moved := func(what string, mut func(*policynet.State)) {
		s2 := st
		s2.Dense = slices.Clone(st.Dense)
		s2.Cards = slices.Clone(st.Cards)
		for i := range s2.Cards {
			s2.Cards[i].Raw = slices.Clone(s2.Cards[i].Raw)
		}
		mut(&s2)
		if run(&s2).Value == got.Value {
			t.Errorf("pod graph: %s does not move the value", what)
		}
	}
	moved("the monarch column", func(s *policynet.State) {
		s.Dense[policynet.DenseWidth+3*policynet.PodSeatWidth+policynet.PodMonarch] = 0
	})
	moved("the attacked-seat column", func(s *policynet.State) {
		s.Cards[0].Raw[policynet.PodEntAttacked0+3], s.Cards[0].Raw[policynet.PodEntAttacked0+1] = 0, 1
	})
	const golden = "testdata/gnn/pod_golden.json"
	if os.Getenv("GORGE_UPDATE_PODGOLDEN") != "" {
		b, _ := json.MarshalIndent(got, "", "  ")
		if err := os.WriteFile(golden, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	var want podGolden
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	ok := near(got.Value, want.Value) && len(got.Scores) == len(want.Scores) && len(got.ValueK) == len(want.ValueK)
	for i := range want.Scores {
		ok = ok && near(got.Scores[i], want.Scores[i])
	}
	for i := range want.ValueK {
		ok = ok && near(got.ValueK[i], want.ValueK[i])
	}
	if !ok {
		t.Errorf("pod graph outputs %+v, golden %+v", got, want)
	}
}
