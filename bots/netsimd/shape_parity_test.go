package netsimd_test

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet"
	"github.com/adams-shaun/gorge/bots/netsimd"
	"github.com/adams-shaun/gorge/bots/policynet"
)

// netShape is one mlp network shape: the trainer's config keys.
type netShape struct {
	d, ff, e, cd, hh int
	layers, pl, vl   int
}

func (s netShape) String() string {
	return fmt.Sprintf("d%d_ff%d_e%d_cd%d_hh%d_L%d_P%d_V%d", s.d, s.ff, s.e, s.cd, s.hh, s.layers, s.pl, s.vl)
}

// writeShapeNet writes a random .dzgw of shape s (dzg.export's layout: the
// tensor names models.MLPNet registers, transposed nothing, LayerNorm eps
// per norm) and returns its path. Weights are N(0, 0.1); LayerNorm gains are
// 1 + N(0, 0.1) so the affine is exercised; the identity table is N(0, 0.3).
func writeShapeNet(t testing.TB, dir string, s netShape, seed uint64) string {
	t.Helper()
	r := rand.New(rand.NewPCG(seed, 99))
	type tensor struct {
		Name   string `json:"name"`
		Shape  []int  `json:"shape"`
		Offset int64  `json:"offset"`
		NBytes int64  `json:"nbytes"`
	}
	var table []tensor
	var body []byte
	add := func(name string, scale, mean float64, shape ...int) {
		n := 1
		for _, d := range shape {
			n *= d
		}
		table = append(table, tensor{name, shape, int64(len(body)), int64(4 * n)})
		for range n {
			body = binary.LittleEndian.AppendUint32(body, math.Float32bits(float32(mean+scale*r.NormFloat64())))
		}
	}
	lin := func(name string, in, out int, bias bool) {
		add(name+".weight", 0.1, 0, out, in)
		if bias {
			add(name+".bias", 0.1, 0, out)
		}
	}
	eps := map[string]float64{}
	norm := func(name string, w int) {
		add(name+".weight", 0.1, 1, w)
		add(name+".bias", 0.1, 0, w)
		eps[name] = 1e-5
	}
	head := func(name string, in, layers int) {
		for i := 0; i <= layers; i++ {
			out := s.hh
			if i == layers {
				out = 1
			}
			lin(fmt.Sprintf("%s.%d", name, 2*i), in, out, true)
			in = out
		}
	}
	add("table.weight", 0.3, 0, policynet.TableRows, s.e)
	lin("card_raw", policynet.EntityRawWidth, s.cd, true)
	lin("card_rows", s.e, s.cd, false)
	lin("card_phi", s.cd, s.cd, true)
	lin("dense_in", policynet.DenseWidth, s.d, true)
	lin("sparse_proj", s.e, s.d, false)
	lin("pool_proj", 8*s.cd, s.d, true)
	for i := 0; i < s.layers; i++ {
		p := fmt.Sprintf("blocks.%d", i)
		norm(p+".norm", s.d)
		lin(p+".fc1", s.d, s.ff, true)
		lin(p+".fc2", s.ff, s.d, true)
	}
	norm("norm", s.d)
	add("opt_in.slots.weight", 0.1, 0, policynet.OptionSlotWidth, s.d)
	lin("opt_in.hproj", s.e, s.d, false)
	lin("opt_in.dense", policynet.OptionDenseWidth, s.d, true)
	add("opt_in.bot", 0.1, 0, s.d)
	lin("ea_proj", s.cd, s.d, true)
	lin("eb_proj", s.cd, s.d, true)
	add("none_a", 0.1, 0, s.cd)
	add("none_b", 0.1, 0, s.cd)
	norm("opt_norm", s.d)
	head("policy", 2*s.d, s.pl)
	head("value", s.d, s.vl)

	h := map[string]any{
		"format": "dzgw-1",
		"arch":   "mlp",
		"config": map[string]float64{"d": float64(s.d), "ff": float64(s.ff), "d_emb": float64(s.e),
			"card_d": float64(s.cd), "head_hidden": float64(s.hh), "layers": float64(s.layers),
			"policy_layers": float64(s.pl), "value_layers": float64(s.vl)},
		"consts": map[string]int{"DENSE_W": policynet.DenseWidth, "RAW_W": policynet.EntityRawWidth,
			"OPT_DENSE_W": policynet.OptionDenseWidth, "TABLE_ROWS": policynet.TableRows,
			"SLOT_ROWS": policynet.OptionSlotWidth, "GROUPS": 4},
		"layernorm_eps": eps,
		"tensors":       table,
	}
	hdr, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	out := append([]byte("DZGW0001"), binary.LittleEndian.AppendUint64(nil, uint64(len(hdr)))...)
	out = append(out, hdr...)
	for len(out)%64 != 0 {
		out = append(out, 0)
	}
	out = append(out, body...)
	path := filepath.Join(dir, s.String()+".dzgw")
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// synthInputs builds n random decisions shaped like the corpus (~15 cards,
// ~140 sparse rows, ~3 options of ~6 slots and ~2 hashed rows each).
func synthInputs(n int, seed uint64) ([]policynet.State, [][]policynet.Option) {
	r := rand.New(rand.NewPCG(seed, 7))
	vec := func(w int) []float32 {
		v := make([]float32, w)
		for i := range v {
			if r.IntN(3) > 0 {
				v[i] = float32(r.NormFloat64())
			}
		}
		return v
	}
	feats := func(k, rows int) []policynet.Feature {
		f := make([]policynet.Feature, k)
		for i := range f {
			f[i] = policynet.Feature{Row: uint16(r.IntN(rows)), Value: float32(r.NormFloat64())}
		}
		return f
	}
	sts := make([]policynet.State, n)
	opts := make([][]policynet.Option, n)
	for i := range sts {
		nc := r.IntN(26)
		cards := make([]policynet.EntityCard, nc)
		for k := range cards {
			cards[k] = policynet.EntityCard{Group: uint8(r.IntN(5)), Raw: vec(policynet.EntityRawWidth), Rows: feats(1+r.IntN(2), policynet.TableRows)}
		}
		sts[i] = policynet.State{Dense: vec(policynet.DenseWidth), Sparse: feats(r.IntN(200), policynet.TableRows), Cards: cards}
		no := r.IntN(9)
		for range no {
			opts[i] = append(opts[i], policynet.Option{
				Slots:   feats(r.IntN(10), policynet.OptionSlotWidth),
				Hashed:  feats(r.IntN(4), policynet.TableRows),
				Dense:   vec(policynet.OptionDenseWidth),
				BotPick: r.IntN(4) == 0,
				EntA:    int32(r.IntN(nc + 1)),
				EntB:    int32(r.IntN(nc + 1)),
			})
		}
	}
	return sts, opts
}

// The netsimd forward against gonet's over random networks of every shape
// the trainer builds -- 1 to 3 trunk blocks, 1 to 3 policy and value head
// layers, widths on the 64-lane fast path and off it -- on random inputs.
// No corpus, no network on disk: it runs everywhere. SIMD and scalar must
// agree bit for bit; both must agree with gonet (float64 layernorm and dot,
// its own summation order) to 1e-5·(1+|logit|): fp32 rounding through a few
// dozen layers, an order of magnitude above the ~3e-6 measured.
func TestShapesMatchGonet(t *testing.T) {
	t.Cleanup(func() { netsimd.ForceScalar = false })
	shapes := []netShape{
		{64, 128, 32, 32, 64, 1, 2, 1}, // the production shape
		{64, 128, 32, 32, 64, 2, 2, 1},
		{64, 128, 32, 32, 64, 3, 2, 1},
		{128, 256, 32, 32, 128, 1, 2, 1},
		{128, 256, 32, 32, 128, 2, 2, 1},
		{128, 256, 32, 32, 128, 3, 2, 1},
		{128, 256, 64, 64, 128, 2, 3, 2},
		{96, 192, 48, 96, 96, 2, 1, 1},
		{256, 512, 32, 32, 256, 2, 2, 1},
		{64, 128, 32, 32, 128, 2, 2, 3},
		{160, 320, 16, 64, 96, 1, 2, 1},
	}
	dir := t.TempDir()
	sts, opts := synthInputs(60, 1)
	gonet.CacheSize = 0
	for si, s := range shapes {
		t.Run(s.String(), func(t *testing.T) {
			path := writeShapeNet(t, dir, s, uint64(si+1))
			ref, err := gonet.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			ref.SetCardCaches(false)
			g, err := netsimd.Load(path, 0)
			if err != nil {
				t.Fatal(err)
			}
			var maxAbs, maxRef float64
			for _, scalar := range []bool{false, true} {
				if !scalar && !netsimd.SIMD() {
					continue
				}
				netsimd.ForceScalar = scalar
				var sc netsimd.Scratch
				for i := range sts {
					a := make([]float32, len(opts[i]))
					b := make([]float32, len(opts[i]))
					va := ref.Eval(&sts[i], opts[i], a)
					vb := g.Eval(&sc, &sts[i], opts[i], b)
					cmp := func(where string, x, y float32) {
						maxAbs = math.Max(maxAbs, math.Abs(float64(x-y)))
						maxRef = math.Max(maxRef, math.Abs(float64(x)))
						if math.IsNaN(float64(y)) || math.Abs(float64(x-y)) > 1e-5*(1+math.Abs(float64(x))) {
							t.Fatalf("scalar=%v rec %d %s: gonet %v netsimd %v", scalar, i, where, x, y)
						}
					}
					cmp("value", va, vb)
					for j := range a {
						cmp(fmt.Sprintf("opt %d", j), a[j], b[j])
					}
				}
			}
			netsimd.ForceScalar = false
			t.Logf("max |gonet - netsimd| %.3g over logits up to %.3g", maxAbs, maxRef)
			if !netsimd.SIMD() {
				return
			}
			// SIMD and scalar bit for bit
			var sa, sb netsimd.Scratch
			for i := range sts {
				a := make([]float32, len(opts[i]))
				b := make([]float32, len(opts[i]))
				netsimd.ForceScalar = false
				va := g.Eval(&sa, &sts[i], opts[i], a)
				netsimd.ForceScalar = true
				vb := g.Eval(&sb, &sts[i], opts[i], b)
				netsimd.ForceScalar = false
				if math.Float32bits(va) != math.Float32bits(vb) {
					t.Fatalf("rec %d: SIMD value %v, scalar %v", i, va, vb)
				}
				for j := range a {
					if math.Float32bits(a[j]) != math.Float32bits(b[j]) {
						t.Fatalf("rec %d opt %d: SIMD %v, scalar %v", i, j, a[j], b[j])
					}
				}
			}
			// and no allocation per eval at this shape
			i := len(sts) / 2
			scores := make([]float32, 16)
			g.Eval(&sa, &sts[i], opts[i], scores)
			if n := testing.AllocsPerRun(20, func() { g.Eval(&sa, &sts[i], opts[i], scores) }); n != 0 {
				t.Fatalf("Eval allocates %v per call", n)
			}
		})
	}
}

// Load names the trunk and head depths it cannot read.
func TestLoadRejectsBadDepths(t *testing.T) {
	dir := t.TempDir()
	for _, s := range []netShape{
		{64, 128, 32, 32, 64, 0, 2, 1},
		{64, 128, 32, 32, 64, 1, 0, 1},
		{64, 128, 32, 32, 64, 1, 2, 0},
	} {
		path := writeShapeNet(t, dir, s, 5)
		if _, err := netsimd.Load(path, 0); err == nil {
			t.Errorf("%v: Load accepted it", s)
		}
	}
}

// The production layout's straight-line trunk (trunkBag1) and the general
// loops (trunkBagN) are the same arithmetic: bit-identical on the corpus
// network and on every random production-layout shape.
func TestOneLayoutMatchesGeneral(t *testing.T) {
	t.Cleanup(func() { netsimd.ForceScalar = false })
	dir := t.TempDir()
	sts, opts := synthInputs(60, 3)
	check := func(name string, g *netsimd.Net, sts []policynet.State, opts [][]policynet.Option) {
		if !netsimd.SetGeneral(g, false) {
			t.Fatalf("%s: not the production layout", name)
		}
		for _, scalar := range []bool{false, true} {
			netsimd.ForceScalar = scalar
			var sc netsimd.Scratch
			for i := range sts {
				a := make([]float32, len(opts[i]))
				b := make([]float32, len(opts[i]))
				netsimd.SetGeneral(g, false)
				va := g.Eval(&sc, &sts[i], opts[i], a)
				netsimd.SetGeneral(g, true)
				vb := g.Eval(&sc, &sts[i], opts[i], b)
				netsimd.SetGeneral(g, false)
				if math.Float32bits(va) != math.Float32bits(vb) {
					t.Fatalf("%s scalar=%v rec %d: one %v general %v", name, scalar, i, va, vb)
				}
				for j := range a {
					if math.Float32bits(a[j]) != math.Float32bits(b[j]) {
						t.Fatalf("%s scalar=%v rec %d opt %d: one %v general %v", name, scalar, i, j, a[j], b[j])
					}
				}
			}
		}
		netsimd.ForceScalar = false
	}
	for i, s := range []netShape{{64, 128, 32, 32, 64, 1, 2, 1}, {128, 256, 64, 64, 128, 1, 2, 1}, {96, 192, 48, 96, 96, 1, 2, 1}} {
		g, err := netsimd.Load(writeShapeNet(t, dir, s, uint64(40+i)), 0)
		if err != nil {
			t.Fatal(err)
		}
		check(s.String(), g, sts, opts)
	}
	if os.Getenv("NETSIMD_DZGW") != "" {
		recs := corpus(t)
		g := loadSIMD(t, 0)
		if !netsimd.SetGeneral(g, false) {
			return // a multi-block or deep-head network: no production layout
		}
		cs := make([]policynet.State, len(recs))
		co := make([][]policynet.Option, len(recs))
		for i := range recs {
			cs[i], co[i] = recs[i].st, recs[i].opts
		}
		check("corpus", g, cs, co)
	}
}
