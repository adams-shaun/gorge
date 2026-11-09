package gonet_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet"
	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/policynet"
)

// TestOptionWidth24NetUnchanged: a network exported before the relative-seat
// option columns (OPT_DENSE_W 24, carrying the 24-wide entity hash) loads and
// reads only the first 24 columns, so an option row with the new columns
// set scores bit-identically to one without them, and to a 28-wide net whose
// new columns' weights are zero (gonettest draws both from one sequence).
func TestOptionWidth24NetUnchanged(t *testing.T) {
	old := gonet.CacheSize
	gonet.CacheSize = 0
	t.Cleanup(func() { gonet.CacheSize = old })

	dir := t.TempDir()
	hash24 := fmt.Sprintf("%016x", policynet.EncoderHashForV1Options(policynet.FeaturesEntity))
	hash28 := fmt.Sprintf("%016x", policynet.EncoderHashForV1Dense(policynet.FeaturesEntity))
	narrowPath, err := gonettest.WriteWith(dir, "n24.dzgw", map[string]any{"DENSE_W": policynet.DenseWidthV1, "OPT_DENSE_W": policynet.OptionDenseWidthV1, "encoder_hash": hash24})
	if err != nil {
		t.Fatal(err)
	}
	widePath, err := gonettest.WriteWith(dir, "w28.dzgw", map[string]any{"DENSE_W": policynet.DenseWidthV1, "encoder_hash": hash28})
	if err != nil {
		t.Fatal(err)
	}
	narrow, err := gonet.Load(narrowPath)
	if err != nil {
		t.Fatalf("24-wide Load: %v", err)
	}
	wide, err := gonet.Load(widePath)
	if err != nil {
		t.Fatalf("28-wide Load: %v", err)
	}
	if !narrow.EncoderHashChecked() || !wide.EncoderHashChecked() {
		t.Fatal("encoder hash not checked")
	}
	// the current hash on a 24-wide net is refused
	bad, err := gonettest.WriteWith(dir, "bad.dzgw", map[string]any{"OPT_DENSE_W": policynet.OptionDenseWidthV1, "encoder_hash": hash28})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gonet.Load(bad); err == nil {
		t.Fatal("a 24-wide net under the 28-wide encoder hash loaded")
	}

	st, base := sampleState()
	full := make([]policynet.Option, len(base))
	zeroed := make([]policynet.Option, len(base))
	short := make([]policynet.Option, len(base))
	for i := range base {
		d := make([]float32, policynet.OptionDenseWidth)
		for j := range d {
			d[j] = float32((i+j)%5) / 4
		}
		d[policynet.OptionDenseWidthV1+(i%4)] = 1 // a relative-seat one-hot
		full[i], zeroed[i], short[i] = base[i], base[i], base[i]
		full[i].Dense = d
		zeroed[i].Dense = append([]float32(nil), d...)
		clear(zeroed[i].Dense[policynet.OptionDenseWidthV1:])
		short[i].Dense = d[:policynet.OptionDenseWidthV1]
	}
	run := func(g *gonet.Net, opts []policynet.Option) (float32, []float32) {
		s := make([]float32, len(opts))
		return g.Eval(&st, opts, s), s
	}
	wantV, wantS := run(narrow, short)
	for _, c := range []struct {
		name string
		g    *gonet.Net
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
