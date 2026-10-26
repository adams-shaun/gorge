package netsimd

import (
	"math"
	"math/rand/v2"
	"testing"
)

// wideWidths are the multiples of 16 the width-generic AVX-512 kernels take:
// every block-size boundary of bagNAVX512 (128/64/16) and of the 4-chunk
// reductions, below, at and past each.
var wideWidths = []int{16, 32, 48, 64, 80, 96, 112, 128, 144, 160, 192, 224, 256, 320, 384, 512}

// Every width-generic assembly kernel is bit-identical to its scalar twin at
// every multiple-of-16 width (the reduction trees are spelled the same; the
// bag and the elementwise kernels are per-lane chains).
func TestWideKernelsMatchScalar(t *testing.T) {
	if !hasAVX512 {
		t.Skip("no AVX-512 on this CPU")
	}
	t.Cleanup(func() { ForceScalar = false })
	r := rand.New(rand.NewPCG(11, 12))
	both := func(f func()) {
		ForceScalar = false
		f()
		ForceScalar = true
		f()
		ForceScalar = false
	}
	for _, n := range wideWidths {
		// bag, with rows at and past the limit, repeats, and an empty bag
		const rows = 300
		table := randF(r, rows*n)
		for _, nf := range []int{0, 1, 7, 40} {
			feats := make([]feat, max(nf, 1))
			for i := range feats {
				feats[i] = feat{Row: uint16(r.IntN(rows + 20)), Value: float32(r.NormFloat64())}
			}
			y0 := randF(r, n)
			ya, ys := aligned(n), aligned(n)
			copy(ya, y0)
			copy(ys, y0)
			ForceScalar = false
			bag(&ya[0], &table[0], &feats[0], nf, rows, n)
			ForceScalar = true
			bag(&ys[0], &table[0], &feats[0], nf, rows, n)
			ForceScalar = false
			if i, d := same(ya, ys); i >= 0 {
				t.Errorf("bag width=%d feats=%d: lane %d differs by %g", n, nf, i, d)
			}
		}

		// layernorm, out of place and in place, on a shifted input
		x, w, b := randF(r, n), randF(r, n), randF(r, n)
		for i := range x {
			x[i] = x[i]*3 + 1.5
		}
		var outs [2][]float32
		k := 0
		both(func() {
			y := aligned(n)
			lnN(&y[0], &x[0], &w[0], &b[0], 1e-5, n)
			outs[k] = y
			k++
		})
		if i, d := same(outs[0], outs[1]); i >= 0 {
			t.Errorf("lnN n=%d: lane %d differs by %g (%v vs %v)", n, i, d, outs[0][i], outs[1][i])
		}
		yi := aligned(n)
		copy(yi, x)
		lnN(&yi[0], &yi[0], &w[0], &b[0], 1e-5, n)
		if i, d := same(yi, outs[0]); i >= 0 {
			t.Errorf("lnN n=%d in place: lane %d differs by %g", n, i, d)
		}

		// dot
		var dots [2]float32
		k = 0
		both(func() { dots[k] = dotN(&x[0], &w[0], n); k++ })
		if math.Float32bits(dots[0]) != math.Float32bits(dots[1]) {
			t.Errorf("dotN n=%d: %v vs %v", n, dots[0], dots[1])
		}

		// addv
		ya, ys := randF(r, n), aligned(n)
		copy(ys, ya)
		ForceScalar = false
		addvN(&ya[0], &x[0], n)
		ForceScalar = true
		addvN(&ys[0], &x[0], n)
		ForceScalar = false
		if i, d := same(ya, ys); i >= 0 {
			t.Errorf("addvN n=%d: lane %d differs by %g", n, i, d)
		}

		// pool
		sa, ma := randF(r, n), randF(r, n)
		ss, ms := aligned(n), aligned(n)
		copy(ss, sa)
		copy(ms, ma)
		ForceScalar = false
		poolAccN(&sa[0], &ma[0], &x[0], n)
		ForceScalar = true
		poolAccN(&ss[0], &ms[0], &x[0], n)
		ForceScalar = false
		if i, d := same(sa, ss); i >= 0 {
			t.Errorf("poolAccN n=%d sum: lane %d differs by %g", n, i, d)
		}
		if i, d := same(ma, ms); i >= 0 {
			t.Errorf("poolAccN n=%d max: lane %d differs by %g", n, i, d)
		}
	}
}

// At 64 lanes the width-generic kernels are the 64-lane ones: the same
// reduction tree, so the same bits (on inputs with no signed zeros, the one
// place a zero-initialised accumulator and a first-chunk sum can differ).
func TestWideKernelsAt64AreTheFastPath(t *testing.T) {
	t.Cleanup(func() { ForceScalar = false })
	r := rand.New(rand.NewPCG(13, 14))
	for _, force := range []bool{false, true} {
		if !force && !hasAVX512 {
			continue
		}
		ForceScalar = force
		for range 20 {
			x, w, b := randF(r, 64), randF(r, 64), randF(r, 64)
			ya, yb := aligned(64), aligned(64)
			ln64(&ya[0], &x[0], &w[0], &b[0], 1e-5)
			lnN(&yb[0], &x[0], &w[0], &b[0], 1e-5, 64)
			if i, d := same(ya, yb); i >= 0 {
				t.Fatalf("ForceScalar=%v: lnN(64) lane %d differs from ln64 by %g", force, i, d)
			}
			if a, c := dot64(&x[0], &w[0]), dotN(&x[0], &w[0], 64); math.Float32bits(a) != math.Float32bits(c) {
				t.Fatalf("ForceScalar=%v: dotN(64) %v, dot64 %v", force, c, a)
			}
		}
	}
}

// The fp32 layernorm against a float64 reference at the wide widths: the
// tolerance the parity tests against gonet (which normalises in float64)
// inherit.
func TestWideLayerNormMatchesFloat64(t *testing.T) {
	r := rand.New(rand.NewPCG(15, 16))
	var worst float64
	for _, n := range wideWidths {
		for range 50 {
			x, w, b := randF(r, n), randF(r, n), randF(r, n)
			for i := range x {
				x[i] = x[i]*4 + 2
			}
			y := aligned(n)
			lnN(&y[0], &x[0], &w[0], &b[0], 1e-5, n)
			var m, q float64
			for _, v := range x {
				m += float64(v)
			}
			m /= float64(n)
			for _, v := range x {
				q += (float64(v) - m) * (float64(v) - m)
			}
			inv := 1 / math.Sqrt(q/float64(n)+1e-5)
			for i, v := range x {
				want := (float64(v)-m)*inv*float64(w[i]) + float64(b[i])
				e := math.Abs(float64(y[i])-want) / (1 + math.Abs(want))
				worst = math.Max(worst, e)
			}
		}
	}
	t.Logf("worst |fp32 LN - float64 LN| / (1+|ref|) = %.3g", worst)
	if worst > 2e-6 {
		t.Fatalf("fp32 layernorm drifts %.3g from float64", worst)
	}
}
