package gonet_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet"
	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/policynet"
)

// TestDenseWidth68NetUnchanged: a network exported before the Commander
// block (DENSE_W 68, carrying the 68-wide entity hash) loads and reads only
// the first 68 dense columns, so a state with the new Commander block set
// scores bit-identically to the same state with those columns absent.
func TestDenseWidth68NetUnchanged(t *testing.T) {
	old := gonet.CacheSize
	gonet.CacheSize = 0
	t.Cleanup(func() { gonet.CacheSize = old })

	dir := t.TempDir()
	hash68 := fmt.Sprintf("%016x", policynet.EncoderHashForV1Dense(policynet.FeaturesEntity))
	path, err := gonettest.WriteWith(dir, "cmd68.dzgw", map[string]any{
		"DENSE_W":      policynet.DenseWidthV1,
		"encoder_hash": hash68,
	})
	if err != nil {
		t.Fatal(err)
	}
	g, err := gonet.Load(path)
	if err != nil {
		t.Fatalf("68-wide Load: %v", err)
	}
	if !g.EncoderHashChecked() {
		t.Fatal("68-wide net loaded without checking its encoder hash")
	}

	st, opts := sampleState()
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

	run := func(s *policynet.State) (float32, []float32) {
		scores := make([]float32, len(opts))
		return g.Eval(s, opts, scores), scores
	}
	wantV, wantS := run(&st)
	for _, tc := range []struct {
		name string
		s    *policynet.State
	}{{"zeroed Commander block", &zeroed}, {"absent Commander block", &short}} {
		v, s := run(tc.s)
		if math.Float32bits(v) != math.Float32bits(wantV) {
			t.Fatalf("%s: value %v vs %v", tc.name, v, wantV)
		}
		for i := range wantS {
			if math.Float32bits(s[i]) != math.Float32bits(wantS[i]) {
				t.Fatalf("%s: score %d %v vs %v", tc.name, i, s[i], wantS[i])
			}
		}
	}
}
