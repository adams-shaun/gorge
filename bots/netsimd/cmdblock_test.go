package netsimd_test

import (
	"math"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/netsimd"
	"github.com/adams-shaun/gorge/bots/policynet"
)

// TestDenseWidth68NetUnchanged: netsimd loads a pre-Commander-block network
// (DENSE_W 68) and reads only the first 68 dense columns, so a state with the
// new Commander block set scores bit-identically to one without it.
func TestDenseWidth68NetUnchanged(t *testing.T) {
	dir := t.TempDir()
	path, err := gonettest.WriteShape(dir, "cmd68.dzgw", gonettest.Wide, policynet.EntityRawWidth,
		map[string]any{"DENSE_W": policynet.DenseWidthV1})
	if err != nil {
		t.Fatal(err)
	}
	g, err := netsimd.Load(path, 0)
	if err != nil {
		t.Fatalf("68-wide Load: %v", err)
	}

	st := policynet.State{Dense: make([]float32, policynet.DenseWidth), Sparse: []policynet.Feature{{Row: 3, Value: 1}}}
	for i := range st.Dense {
		st.Dense[i] = float32(i%7) / 7
	}
	commanderNonZero := false
	for _, x := range st.Dense[policynet.DenseWidthV1:] {
		if x != 0 {
			commanderNonZero = true
		}
	}
	if !commanderNonZero {
		t.Fatal("fixture has no non-zero Commander block; the comparison proves nothing")
	}
	zeroed := st
	zeroed.Dense = append([]float32(nil), st.Dense...)
	clear(zeroed.Dense[policynet.DenseWidthV1:])
	short := st
	short.Dense = st.Dense[:policynet.DenseWidthV1]

	var sc netsimd.Scratch
	run := func(s *policynet.State) float32 {
		return g.Eval(&sc, s, nil, nil)
	}
	want := run(&st)
	for _, tc := range []struct {
		name string
		s    *policynet.State
	}{{"zeroed Commander block", &zeroed}, {"absent Commander block", &short}} {
		if got := run(tc.s); math.Float32bits(got) != math.Float32bits(want) {
			t.Fatalf("%s: value %v vs %v", tc.name, got, want)
		}
	}
}
