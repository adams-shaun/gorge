package gonet

import (
	"math"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/internal/simdk"
	"github.com/adams-shaun/gorge/bots/policynet"
)

// valueKState is a small encoded state with cards in three groups.
func valueKState() (policynet.State, []policynet.Option) {
	dense := make([]float32, policynet.DenseWidth)
	for i := range dense {
		dense[i] = float32(i%7) / 7
	}
	raw := func(k int) []float32 {
		r := make([]float32, policynet.EntityRawWidth)
		for i := range r {
			r[i] = float32((i+k)%5) / 5
		}
		return r
	}
	st := policynet.State{
		Dense:  dense,
		Sparse: []policynet.Feature{{Row: 3, Value: 1}, {Row: 900, Value: 2}},
		Cards: []policynet.EntityCard{
			{Group: 0, Raw: raw(0), Rows: []policynet.Feature{{Row: 10, Value: 1}}},
			{Group: 1, Raw: raw(1), Rows: []policynet.Feature{{Row: 11, Value: 1}, {Row: 12, Value: 1}}},
			{Group: 2, Raw: raw(2)},
		},
	}
	od := make([]float32, policynet.OptionDenseWidth)
	opts := []policynet.Option{
		{Slots: []policynet.Feature{{Row: 1, Value: 1}}, Hashed: []policynet.Feature{{Row: 10, Value: 1}}, Dense: od},
		{Slots: []policynet.Feature{{Row: 5, Value: 1}}, Dense: od, EntA: 1},
	}
	return st, opts
}

// mirrorHead is a K-way head that answers the scalar head's logit in every
// component: the hidden layers copied, the last layer's one row repeated k
// times. Its logits are the scalar logit bit for bit when the forward runs
// the same arithmetic for each output.
func mirrorHead(value []gnLinear, k int, pack bool) []gnLinear {
	out := slices.Clone(value)
	last := value[len(value)-1]
	l := gnLinear{in: last.in, out: k, w: make([]float32, k*last.in), b: make([]float32, k)}
	for o := 0; o < k; o++ {
		copy(l.w[o*last.in:(o+1)*last.in], last.w[:last.in])
		l.b[o] = last.b[0]
	}
	if pack {
		l.pk = simdk.Pack(l.w, l.b, l.in, l.out)
	}
	out[len(out)-1] = l
	return out
}

// TestValueKHeadLeavesTheRestUnchanged: a network written with a K-way head
// (gonettest Layout.ValueK) loads with ValueK set, answers every option
// logit and the scalar value exactly as the same network without the head,
// and its ValueVec is a distribution over K outcomes; the scalar network
// has no vector (ValueK 0, ValueVec nil).
func TestValueKHeadLeavesTheRestUnchanged(t *testing.T) {
	dir := t.TempDir()
	lay := gonettest.DefaultLayout // a copy: ValueK is set on it below
	scalarPath, err := gonettest.WriteLayout(dir, "s.dzgw", gonettest.Small, policynet.EntityRawWidth, nil, lay)
	if err != nil {
		t.Fatal(err)
	}
	lay.ValueK = 5
	kPath, err := gonettest.WriteLayout(dir, "k.dzgw", gonettest.Small, policynet.EntityRawWidth, nil, lay)
	if err != nil {
		t.Fatal(err)
	}
	s, err := Load(scalarPath)
	if err != nil {
		t.Fatal(err)
	}
	k, err := Load(kPath)
	if err != nil {
		t.Fatal(err)
	}
	if s.ValueK() != 0 || k.ValueK() != 5 {
		t.Fatalf("ValueK: scalar %d, K-way %d; want 0 and 5", s.ValueK(), k.ValueK())
	}
	st, opts := valueKState()
	ss, ks := make([]float32, len(opts)), make([]float32, len(opts))
	if sv, kv := s.Eval(&st, opts, ss), k.Eval(&st, opts, ks); sv != kv || !slices.Equal(ss, ks) {
		t.Fatalf("the K-way head moved the rest: value %v vs %v, scores %v vs %v", sv, kv, ss, ks)
	}
	if v := s.ValueVec(st); v != nil {
		t.Errorf("scalar network ValueVec = %v, want nil", v)
	}
	p := k.ValueVec(st)
	sum := 0.0
	for _, x := range p {
		if !(x > 0 && x < 1) {
			t.Errorf("ValueVec %v: an entry outside (0,1)", p)
		}
		sum += float64(x)
	}
	if len(p) != 5 || math.Abs(sum-1) > 1e-6 {
		t.Errorf("ValueVec %v (sum %v): want 5 probabilities summing to 1", p, sum)
	}
	vk := make([]float32, 5)
	if v := k.EvalValueVec(&st, vk); v != k.Eval(&st, nil, nil) {
		t.Errorf("EvalValueVec's value logit %v, Eval's %v", v, k.Eval(&st, nil, nil))
	}
	m, err := Model(kPath)
	if err != nil {
		t.Fatal(err)
	}
	if m.ValueK != 5 || !slices.Equal(m.ValueVec(st), p) {
		t.Errorf("Model: ValueK %d, ValueVec %v; want 5 and %v", m.ValueK, m.ValueVec(st), p)
	}
}

// TestEvalValueVecReadsTheHead pins both forwards' K-way plumbing: a head
// that mirrors the scalar head answers the scalar logit in every
// component, through the mlp's head loop and the graph network's batched
// GEMM head, and a warm EvalValueVec allocates nothing.
func TestEvalValueVecReadsTheHead(t *testing.T) {
	path, err := gonettest.WriteLayout(t.TempDir(), "m.dzgw", gonettest.Small, policynet.EntityRawWidth, nil,
		gonettest.Layout{Layers: 1, PolicyLayers: 1, ValueLayers: 2})
	if err != nil {
		t.Fatal(err)
	}
	mlp, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	mlp.valueK, mlp.valueKHead = 4, mirrorHead(mlp.value, 4, false)
	gnn, err := Load("testdata/gnn/gnn.dzgw")
	if err != nil {
		t.Fatal(err)
	}
	gnn.gnn.valueK, gnn.gnn.valueKHead = 4, mirrorHead(gnn.gnn.value, 4, true)
	st, _ := valueKState()
	for _, c := range []struct {
		name string
		g    *Net
	}{{"mlp", mlp}, {"gnn", gnn}} {
		t.Run(c.name, func(t *testing.T) {
			want := c.g.Eval(&st, nil, nil)
			vk := make([]float32, 4)
			if v := c.g.EvalValueVec(&st, vk); v != want {
				t.Errorf("value logit %v, Eval's %v", v, want)
			}
			for i, x := range vk {
				if math.Abs(float64(x-want)) > 1e-5 {
					t.Errorf("component %d: %v, the scalar head's %v", i, x, want)
				}
			}
			if a := testing.AllocsPerRun(20, func() { c.g.EvalValueVec(&st, vk) }); a != 0 {
				t.Errorf("EvalValueVec allocates %v per call", a)
			}
		})
	}
}

// TestValueKRefusesTooFewOutcomes: a head of fewer than three outcomes
// (two seats and no winner) is a malformed export.
func TestValueKRefusesTooFewOutcomes(t *testing.T) {
	path, err := gonettest.WriteLayout(t.TempDir(), "k2.dzgw", gonettest.Small, policynet.EntityRawWidth, nil,
		gonettest.Layout{Layers: 1, PolicyLayers: 1, ValueLayers: 1, ValueK: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("a value_k 2 network loaded")
	}
}
