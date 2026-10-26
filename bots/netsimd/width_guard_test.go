package netsimd

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/policynet"
)

// TestPoolAccWideWidth pins the pooling kernel over the whole card width:
// the 32-lane assembly kernel is the fast path, and every other width must
// still add every lane to both pools (a 64- or 128-wide card vector whose
// upper lanes were dropped would silently pool zeros).
func TestPoolAccWideWidth(t *testing.T) {
	t.Cleanup(func() { ForceScalar = false })
	for _, force := range []bool{true, false} {
		ForceScalar = force
		for _, cd := range []int{32, 64, 128} {
			sum := make([]float32, cd)
			mx := make([]float32, cd)
			c := make([]float32, cd)
			for i := range c {
				c[i] = float32(i + 1)
			}
			poolAccW(&sum[0], &mx[0], &c[0], cd)
			for i := range c {
				if want := 0.25 * c[i]; sum[i] != want {
					t.Fatalf("ForceScalar=%v cd=%d sum lane %d = %v, want %v", force, cd, i, sum[i], want)
				}
				if mx[i] != c[i] {
					t.Fatalf("ForceScalar=%v cd=%d max lane %d = %v, want %v", force, cd, i, mx[i], c[i])
				}
			}
		}
	}
}

// TestTrunkPoolsEveryCardLane drives the pooling step through trunkBag with
// a card vector whose last lane is its largest value and a pool projection
// that reads only that lane: the returned value is zero if the pool kernel
// stops at 32 lanes. This is the call site, not just the kernel.
func TestTrunkPoolsEveryCardLane(t *testing.T) {
	t.Cleanup(func() { ForceScalar = false })
	const d, e, ff = 64, 32, 64
	for _, cd := range []int{64, 128} {
		mkLin := func(w []float32, in, out int, b []float32) lin {
			l := lin{in: in, out: out, outPad: roundUp(out, 32)}
			l.wt = aligned(in * l.outPad)
			l.b = aligned(l.outPad)
			for o := 0; o < out; o++ {
				for i := 0; i < in; i++ {
					l.wt[i*l.outPad+o] = w[o*in+i]
				}
			}
			copy(l.b, b)
			return l
		}
		zero := func(in, out int) lin {
			return mkLin(make([]float32, out*in), in, out, make([]float32, out))
		}
		ones := func(n int) []float32 {
			v := make([]float32, n)
			for i := range v {
				v[i] = 1
			}
			return v
		}
		g := &Net{d: d, ff: ff, e: e, cd: cd, hh: d}
		poolIn := poolW * cd
		w := make([]float32, d*poolIn)
		w[groups*cd+cd-1] = 1 // output 0 reads the group-0 max pool's last lane
		g.poolProj = mkLin(w, poolIn, d, make([]float32, d))
		g.denseIn = zero(denseW, d)
		g.sparseProj = zero(e, d)
		blk := block{norm: norm{w: ones(d), b: make([]float32, d), eps: 1e-5}, fc1: zero(d, ff), fc2: zero(ff, d)}
		blk.fc1.wt[0] = 1
		blk.fc2.wt[0] = 1
		g.blocks = []block{blk}
		g.fnorm = norm{w: ones(d), b: make([]float32, d), eps: 1e-5}
		g.val0 = zero(d, d)
		g.val0.wt[0] = 1
		g.valOutW = make([]float32, d)
		g.valOutW[0] = 1

		sc := &Scratch{ent: make([]float32, cd), groups: []uint8{0}}
		for i := range sc.ent {
			sc.ent[i] = float32(i + 1)
		}
		if v := g.trunkBag(sc, 1, make([]float32, denseW), nil, nil, nil, nil); v == 0 {
			t.Fatalf("cd=%d: pooled value 0, the card vector's upper lanes never reached the pool", cd)
		}
	}
}

// TestLoadRejectsUnpaddedWidths pins Load's shape guard: gemvT indexes its
// transposed tensor with the padded out width, so d, ff, hh and cd must be
// multiples of 32 or the kernel reads a different stride than mk wrote.
func TestLoadRejectsUnpaddedWidths(t *testing.T) {
	cases := []struct {
		name             string
		d, ff, e, cd, hh int
	}{
		{"d", 100, 64, 32, 32, 64},
		{"ff", 64, 100, 32, 32, 64},
		{"hh", 64, 64, 32, 32, 100},
		{"cd", 64, 64, 32, 100, 64},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeHeaderOnlyNet(t, tc.d, tc.ff, tc.e, tc.cd, tc.hh)
			_, err := Load(path, 0)
			if err == nil || !strings.Contains(err.Error(), "multiples of 32") {
				t.Fatalf("Load(%s=%d) error = %v, want the multiple-of-32 shape guard", tc.name, map[string]int{
					"d": tc.d, "ff": tc.ff, "hh": tc.hh, "cd": tc.cd,
				}[tc.name], err)
			}
		})
	}
}

// writeHeaderOnlyNet writes a .dzgw file whose header parses and whose
// constants are valid but which carries no tensors: Load reaches the shape
// guard before it reads any tensor.
func writeHeaderOnlyNet(t *testing.T, d, ff, e, cd, hh int) string {
	t.Helper()
	h := header{
		Format: "dzgw-1",
		Arch:   "mlp",
		Config: map[string]float64{"d": float64(d), "ff": float64(ff), "d_emb": float64(e),
			"card_d": float64(cd), "head_hidden": float64(hh), "layers": 1,
			"policy_layers": 2, "value_layers": 1},
		Consts: map[string]int{"RAW_W": policynet.EntityRawWidth, "DENSE_W": denseW,
			"OPT_DENSE_W": optDense, "TABLE_ROWS": tableRows, "SLOT_ROWS": slotRows, "GROUPS": groups},
	}
	body, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	raw := make([]byte, 16+len(body))
	copy(raw, magic)
	binary.LittleEndian.PutUint64(raw[8:], uint64(len(body)))
	copy(raw[16:], body)
	path := filepath.Join(t.TempDir(), "header-only.dzgw")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
