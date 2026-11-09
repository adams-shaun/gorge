package netsimd_test

import (
	"math"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/netsimd"
	"github.com/adams-shaun/gorge/bots/policynet"
)

// TestOptionWidth24NetUnchanged: netsimd loads a pre-relative-seat network
// (OPT_DENSE_W 24) and it reads only the first 24 option columns, so a row
// with the new columns set scores bit-identically to one without them and to
// a 28-wide net whose new columns' weights are zero.
func TestOptionWidth24NetUnchanged(t *testing.T) {
	dir := t.TempDir()
	narrowPath, err := gonettest.WriteShape(dir, "n24.dzgw", gonettest.Wide, policynet.EntityRawWidth, map[string]any{"DENSE_W": policynet.DenseWidthV1, "OPT_DENSE_W": policynet.OptionDenseWidthV1})
	if err != nil {
		t.Fatal(err)
	}
	widePath, err := gonettest.WriteShape(dir, "w28.dzgw", gonettest.Wide, policynet.EntityRawWidth, map[string]any{"DENSE_W": policynet.DenseWidthV1})
	if err != nil {
		t.Fatal(err)
	}
	narrow, err := netsimd.Load(narrowPath, 0)
	if err != nil {
		t.Fatalf("24-wide Load: %v", err)
	}
	wide, err := netsimd.Load(widePath, 0)
	if err != nil {
		t.Fatalf("28-wide Load: %v", err)
	}
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
	st := policynet.State{Dense: dense, Sparse: []policynet.Feature{{Row: 3, Value: 1}},
		Cards: []policynet.EntityCard{
			{Group: 0, Raw: raw(0), Rows: []policynet.Feature{{Row: 10, Value: 1}}},
			{Group: 1, Raw: raw(1), Rows: []policynet.Feature{{Row: 11, Value: 1}}},
		}}
	const n = 4
	full := make([]policynet.Option, n)
	zeroed := make([]policynet.Option, n)
	short := make([]policynet.Option, n)
	for i := 0; i < n; i++ {
		d := make([]float32, policynet.OptionDenseWidth)
		for j := range d {
			d[j] = float32((i+j)%5) / 4
		}
		d[policynet.OptionDenseWidthV1+i] = 1 // a relative-seat one-hot
		o := policynet.Option{Slots: []policynet.Feature{{Row: uint16(1 + i), Value: 1}}, Hashed: []policynet.Feature{{Row: uint16(20 + i), Value: 1}}}
		full[i], zeroed[i], short[i] = o, o, o
		full[i].Dense = d
		zeroed[i].Dense = append([]float32(nil), d...)
		clear(zeroed[i].Dense[policynet.OptionDenseWidthV1:])
		short[i].Dense = d[:policynet.OptionDenseWidthV1]
	}
	run := func(g *netsimd.Net, opts []policynet.Option) (float32, []float32) {
		var sc netsimd.Scratch
		s := make([]float32, len(opts))
		return g.Eval(&sc, &st, opts, s), s
	}
	wantV, wantS := run(narrow, short)
	for _, c := range []struct {
		name string
		g    *netsimd.Net
		opts []policynet.Option
	}{{"24 full", narrow, full}, {"24 zeroed", narrow, zeroed}, {"28 zeroed", wide, zeroed}, {"28 full", wide, full}} {
		v, s := run(c.g, c.opts)
		if math.Float32bits(v) != math.Float32bits(wantV) {
			t.Errorf("%s: value %v vs %v", c.name, v, wantV)
		}
		for i := range s {
			if math.Float32bits(s[i]) != math.Float32bits(wantS[i]) {
				t.Errorf("%s: score %d %v vs %v", c.name, i, s[i], wantS[i])
			}
		}
	}
}
