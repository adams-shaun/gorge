package netsimd_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"math/rand/v2"
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet"
	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/netsimd"
	"github.com/adams-shaun/gorge/bots/policynet"
)

const extSlots = 512

// extShape is one test network: sh is its widths, prod says whether it uses
// the production head layout (one trunk block, policy_layers 2, value_layers
// 1) that netsimd.Load runs through trunkBag1.
type extShape struct {
	name string
	sh   gonettest.Shape
	prod bool
}

var extShapes = []extShape{
	{"d64fast", gonettest.Wide, true},
	{"d96generic", gonettest.Shape{D: 96, FF: 96, E: 32, CD: 32, HH: 96}, false},
}

// writeBase writes fx's network without EXT_W; writeExt writes it with the
// extension block. The layouts match, so the non-ext tensors are identical.
func writeBase(t *testing.T, dir, name string, fx extShape) string {
	t.Helper()
	var p string
	var err error
	if fx.prod {
		p, err = gonettest.WriteLayout(dir, name, fx.sh, policynet.EntityRawWidth, nil, gonettest.ProductionLayout)
	} else {
		p, err = gonettest.WriteShape(dir, name, fx.sh, policynet.EntityRawWidth, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func writeExt(t *testing.T, dir, name string, fx extShape, w []float32) string {
	t.Helper()
	var p string
	var err error
	if fx.prod {
		p, err = gonettest.WriteExtLayout(dir, name, fx.sh, extSlots, w, gonettest.ProductionLayout)
	} else {
		p, err = gonettest.WriteExt(dir, name, fx.sh, extSlots, w)
	}
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func randExt(r *rand.Rand) []policynet.Feature {
	f := make([]policynet.Feature, 1+r.IntN(40))
	for i := range f {
		f[i] = policynet.Feature{Row: uint16(r.IntN(extSlots)), Value: r.Float32()}
	}
	return f
}

// evalExt runs st through the card-vector path (EvalBagExt) with ext.
func evalExt(g *netsimd.Net, sc *netsimd.Scratch, st *policynet.State, ext []policynet.Feature, opts []policynet.Option) (float32, []float32) {
	in := make([]netsimd.CardIn, len(st.Cards))
	for k := range st.Cards {
		c := &st.Cards[k]
		in[k] = netsimd.CardIn{Group: c.Group, Raw: c.Raw, Rows: c.Rows}
	}
	bag := make([]float32, g.BagWidth())
	g.BagAdd(bag, st.Sparse)
	sc2 := make([]float32, len(opts))
	return g.EvalBagExt(sc, in, st.Dense, bag, ext, opts, sc2), sc2
}

func randWeights(r *rand.Rand, n int, scale float32) []float32 {
	w := make([]float32, n)
	for i := range w {
		w[i] = (r.Float32()*2 - 1) * scale
	}
	return w
}

func TestExtNetZeroWeightsEqualsBase(t *testing.T) {
	dir := t.TempDir()
	sts, opts := synthInputs(50, 11)
	r := rand.New(rand.NewPCG(5, 5))
	for _, fx := range extShapes {
		name, sh := fx.name, fx.sh
		base := writeBase(t, dir, name+"-base.dzgw", fx)
		extp := writeExt(t, dir, name+"-ext.dzgw", fx, make([]float32, sh.D*extSlots))
		gb, err := netsimd.Load(base, 0)
		if err != nil {
			t.Fatal(err)
		}
		ge, err := netsimd.Load(extp, 0)
		if err != nil {
			t.Fatal(err)
		}
		if gb.ExtWidth() != 0 || ge.ExtWidth() != extSlots {
			t.Fatalf("%s: ExtWidth base %d ext %d", name, gb.ExtWidth(), ge.ExtWidth())
		}
		var sa, sb netsimd.Scratch
		for i := range sts {
			ext := randExt(r)
			va, a := evalExt(gb, &sa, &sts[i], ext, opts[i])
			vb, b := evalExt(ge, &sb, &sts[i], ext, opts[i])
			if math.Float32bits(va) != math.Float32bits(vb) {
				t.Fatalf("%s rec %d: base %v ext %v", name, i, va, vb)
			}
			for j := range a {
				if math.Float32bits(a[j]) != math.Float32bits(b[j]) {
					t.Fatalf("%s rec %d opt %d: base %v ext %v", name, i, j, a[j], b[j])
				}
			}
		}
	}
}

// patchDenseBias adds delta to the dense_in.bias tensor of the .dzgw at path.
func patchDenseBias(t *testing.T, path string, delta []float64) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	hl := int64(binary.LittleEndian.Uint64(raw[8:16]))
	var h struct {
		Tensors []struct {
			Name   string `json:"name"`
			Offset int64  `json:"offset"`
		} `json:"tensors"`
	}
	if err := json.Unmarshal(raw[16:16+hl], &h); err != nil {
		t.Fatal(err)
	}
	start := (16 + hl + 63) / 64 * 64
	for _, tn := range h.Tensors {
		if tn.Name != "dense_in.bias" {
			continue
		}
		for i, dv := range delta {
			at := start + tn.Offset + int64(4*i)
			v := float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[at:]))) + dv
			binary.LittleEndian.PutUint32(raw[at:], math.Float32bits(float32(v)))
		}
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatal("no dense_in.bias")
}

// The ext add equals folding Σ v·ext_in[:, slot] into dense_in's bias: gonet
// (which has no ext) over the patched ext-free net is the reference. The d=64
// fixture uses the production head layout, so both trunk paths run it: the
// straight-line trunkBag1 and the general trunkBagN. The d=96 fixture has no
// production layout and runs trunkBagN only.
func TestExtNetMatchesReference(t *testing.T) {
	dir := t.TempDir()
	sts, opts := synthInputs(6, 21)
	r := rand.New(rand.NewPCG(8, 8))
	for _, fx := range extShapes {
		name, sh := fx.name, fx.sh
		w := randWeights(r, sh.D*extSlots, 0.2)
		extp := writeExt(t, dir, name+"-ext.dzgw", fx, w)
		g, err := netsimd.Load(extp, 0)
		if err != nil {
			t.Fatal(err)
		}
		if g.ExtWidth() != extSlots {
			t.Fatalf("%s: ExtWidth = %d, want %d", name, g.ExtWidth(), extSlots)
		}
		paths := []struct {
			name    string
			general bool
		}{{"trunkBagN", true}}
		if fx.prod {
			paths = []struct {
				name    string
				general bool
			}{{"trunkBag1", false}, {"trunkBagN", true}}
		}
		for _, path := range paths {
			if fx.prod {
				if !netsimd.SetGeneral(g, path.general) {
					t.Fatalf("%s %s: SetGeneral(%v) = false, want the production layout", name, path.name, path.general)
				}
			} else if netsimd.SetGeneral(g, false) {
				t.Fatalf("%s %s: SetGeneral(false) = true for a non-production layout", name, path.name)
			}
			var sc netsimd.Scratch
			for i := range sts {
				ext := randExt(r)
				delta := make([]float64, sh.D)
				for _, f := range ext {
					for o := range delta {
						delta[o] += float64(f.Value) * float64(w[o*extSlots+int(f.Row)])
					}
				}
				base := writeBase(t, dir, name+"-ref.dzgw", fx)
				patchDenseBias(t, base, delta)
				ref, err := gonet.Load(base)
				if err != nil {
					t.Fatal(err)
				}
				ref.SetCardCaches(false)
				want := ref.Eval(&sts[i], opts[i], make([]float32, len(opts[i])))
				got, _ := evalExt(g, &sc, &sts[i], ext, opts[i])
				if math.Abs(float64(want-got)) > 1e-5*(1+math.Abs(float64(want))) {
					t.Fatalf("%s %s rec %d: reference %v netsimd %v", name, path.name, i, want, got)
				}
				// the ext must matter, or the check proves nothing
				if i == 0 {
					plain, _ := evalExt(g, &sc, &sts[i], nil, opts[i])
					if plain == got {
						t.Fatalf("%s %s: ext did not move the value (%v)", name, path.name, got)
					}
				}
			}
		}
	}
	if p := os.Getenv("NETSIMD_EXT_NET"); p != "" {
		g, err := netsimd.Load(p, 0)
		if err != nil {
			t.Fatal(err)
		}
		if g.ExtWidth() != extSlots {
			t.Fatalf("%s: ExtWidth %d", p, g.ExtWidth())
		}
		var sc netsimd.Scratch
		for i := range sts {
			v, _ := evalExt(g, &sc, &sts[i], randExt(r), opts[i])
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				t.Fatalf("%s rec %d: value %v", p, i, v)
			}
		}
	}
}

func TestExtWidthRefused(t *testing.T) {
	// A well-formed 512-wide tensor under an EXT_W of 100: the tensor shape
	// check passes, so only the const guard can refuse the file.
	p, err := gonettest.WriteExt(t.TempDir(), "bad.dzgw", gonettest.Wide, extSlots, make([]float32, 64*extSlots))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	old := []byte(`"EXT_W":512`)
	if bytes.Count(raw, old) != 1 {
		t.Fatalf("header does not hold %s exactly once", old)
	}
	if err := os.WriteFile(p, bytes.Replace(raw, old, []byte(`"EXT_W":100`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = netsimd.Load(p, 0)
	if err == nil || !strings.Contains(err.Error(), "EXT_W = 100") || !strings.Contains(err.Error(), "512") {
		t.Fatalf("Load = %v, want the EXT_W guard naming 100 and 512", err)
	}
}
