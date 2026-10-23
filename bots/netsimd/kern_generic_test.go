package netsimd

import (
	"math"
	"math/rand"
	"testing"

	"github.com/adams-shaun/gorge/bots/policynet"
)

// The width-generic kernels must agree with the fixed-64 ones on the fast
// path's own width, and be self-consistent at other widths. The 64-lane
// comparison runs the assembly against ln64Scalar/dot64Scalar, so with
// ForceScalar set both sides are the fp32 quarter-sum reduction and the test
// is exact; unset, it compares assembly against the same scalar reference the
// existing kern_test.go compares it to, with its documented ~1e-6 tolerance.
func TestWidthGenericMatchesFastPath(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	mk := func(n int) []float32 {
		v := make([]float32, n)
		for i := range v {
			v[i] = float32(rng.NormFloat64())
		}
		return v
	}
	const n = 64
	x, w, b := mk(n), mk(n), mk(n)
	one := make([]float32, n)
	for i := range one {
		one[i] = 1
	}

	for _, force := range []bool{true, false} {
		ForceScalar = force
		fastLn := make([]float32, n)
		genLn := make([]float32, n)
		ln64(&fastLn[0], &x[0], &w[0], &b[0], 1e-5)
		// lnN applies the same weight and bias over the same lanes; the only
		// permitted difference is the accumulation order.
		lnN(&genLn[0], &x[0], &w[0], &b[0], 1e-5, n)
		for i := range genLn {
			if math.Abs(float64(fastLn[i]-genLn[i])) > 1e-5 {
				t.Fatalf("ForceScalar=%v ln lane %d: fast %g generic %g", force, i, fastLn[i], genLn[i])
			}
		}

		fastDot := dot64(&x[0], &w[0])
		genDot := dotN(&x[0], &w[0], n)
		if rel := math.Abs(float64(fastDot-genDot)) / math.Max(1, math.Abs(float64(fastDot))); rel > 1e-5 {
			t.Fatalf("ForceScalar=%v dot: fast %g generic %g (rel %g)", force, fastDot, genDot, rel)
		}

		af, ag := append([]float32(nil), one...), append([]float32(nil), one...)
		addv64(&af[0], &x[0])
		addvN(&ag[0], &x[0], n)
		for i := range af {
			if af[i] != ag[i] {
				t.Fatalf("ForceScalar=%v addv lane %d: %g vs %g", force, i, af[i], ag[i])
			}
		}
	}
	ForceScalar = false
}

// A non-64 width still computes a correct layernorm: zero mean, unit
// variance up to eps, and it round-trips a bias through.
func TestWidthGenericOtherWidths(t *testing.T) {
	for _, n := range []int{32, 128, 384} {
		x := make([]float32, n)
		w := make([]float32, n)
		b := make([]float32, n)
		for i := 0; i < n; i++ {
			x[i] = float32(i) * 0.37
			w[i] = 2
			b[i] = float32(i) * 0.1
		}
		y := make([]float32, n)
		lnN(&y[0], &x[0], &w[0], &b[0], 1e-5, n)
		// y[i] = 2*((x-mean)/sd) + 0.1*i, so de-bias it and check the moments
		var mean, sq float64
		for i := range y {
			v := float64(y[i]) - float64(b[i])
			v /= 2
			mean += v
		}
		mean /= float64(n)
		for i := range y {
			v := (float64(y[i]) - float64(b[i])) / 2
			sq += (v - mean) * (v - mean)
		}
		var0 := sq / float64(n)
		if math.Abs(mean) > 1e-4 || math.Abs(var0-1) > 1e-3 {
			t.Fatalf("n=%d: mean %g var %g, want 0 and ~1", n, mean, var0)
		}
	}
}

// SumRows at a width the assembly does not implement must take the generic
// kernel and agree with the scalar reference: the assembly's fallthrough
// treats an unknown width as 32, which would read past each row.
func TestSumRowsWideWidth(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	const width, rowsN = 128, 16
	table := make([]float32, width*rowsN)
	for i := range table {
		table[i] = float32(rng.NormFloat64())
	}
	feats := []policynet.Feature{{Row: 0, Value: 1}, {Row: 3, Value: 0.5}, {Row: 15, Value: -2}}
	want := make([]float32, width)
	for _, f := range feats {
		for j := 0; j < width; j++ {
			want[j] += f.Value * table[int(f.Row)*width+j]
		}
	}

	for _, force := range []bool{true, false} {
		ForceScalar = force
		got := make([]float32, width)
		SumRows(got, table, width, feats)
		for j := range got {
			if math.Abs(float64(got[j]-want[j])) > 1e-4*math.Max(1, math.Abs(float64(want[j]))) {
				t.Fatalf("ForceScalar=%v lane %d: %g want %g", force, j, got[j], want[j])
			}
		}
	}
	ForceScalar = false
}

// The fast path's own widths still route through the assembly and match.
func TestSumRowsFastWidths(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	for _, width := range []int{32, 64} {
		table := make([]float32, width*8)
		for i := range table {
			table[i] = float32(rng.NormFloat64())
		}
		feats := []policynet.Feature{{Row: 1, Value: 2}, {Row: 7, Value: -1}}
		want := make([]float32, width)
		for _, f := range feats {
			for j := 0; j < width; j++ {
				want[j] += f.Value * table[int(f.Row)*width+j]
			}
		}
		got := make([]float32, width)
		SumRows(got, table, width, feats)
		for j := range got {
			if math.Abs(float64(got[j]-want[j])) > 1e-4*math.Max(1, math.Abs(float64(want[j]))) {
				t.Fatalf("width %d lane %d: %g want %g", width, j, got[j], want[j])
			}
		}
	}
}
