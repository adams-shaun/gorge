package simdk

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
)

func randF(r *rand.Rand, n int) []float32 {
	s := make([]float32, n)
	for i := range s {
		s[i] = float32(r.NormFloat64())
	}
	return s
}

// The GEMM, both paths, against a float64 reference, over row counts that
// exercise every microkernel tail and widths that exercise the column mask
// and strided rows.
func TestGemm(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for _, sh := range [][2]int{{1, 1}, {16, 16}, {100, 256}, {256, 768}, {1024, 256}, {24, 70}, {68, 128}, {7, 3}} {
		in, out := sh[0], sh[1]
		w, b := randF(r, out*in), randF(r, out)
		for _, bias := range []bool{true, false} {
			bb := b
			if !bias {
				bb = nil
			}
			p := Pack(w, bb, in, out)
			for _, n := range []int{1, 2, 3, 4, 5, 6, 7, 11, 13, 22} {
				xs, ys := in+3, out+5
				x := randF(r, n*xs)
				for _, simd := range []bool{false, true} {
					y := randF(r, n*ys)
					keep := append([]float32(nil), y...)
					Gemm(y, ys, x, xs, n, p, simd)
					for i := 0; i < n; i++ {
						for o := 0; o < out; o++ {
							var want float64
							if bias {
								want = float64(b[o])
							}
							for k := 0; k < in; k++ {
								want += float64(w[o*in+k]) * float64(x[i*xs+k])
							}
							got := y[i*ys+o]
							if d := math.Abs(float64(got) - want); d > 1e-4*math.Sqrt(float64(in))*(1+math.Abs(want)) {
								t.Fatalf("in %d out %d n %d simd %v bias %v: y[%d][%d] = %g, want %g", in, out, n, simd, bias, i, o, got, want)
							}
						}
						for o := out; o < ys; o++ {
							if y[i*ys+o] != keep[i*ys+o] {
								t.Fatalf("in %d out %d n %d simd %v: wrote past the row at [%d][%d]", in, out, n, simd, i, o)
							}
						}
					}
				}
			}
		}
	}
}

func TestLayerNorm(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	for _, n := range []int{16, 32, 48, 128, 256, 20} {
		x, w, b := randF(r, n), randF(r, n), randF(r, n)
		for i := range x {
			x[i] = x[i]*3 + 1.5
		}
		want := make([]float32, n)
		LayerNorm(want, x, w, b, 1e-5, false)
		got := append([]float32(nil), x...)
		LayerNorm(got, got, w, b, 1e-5, true) // in place
		for i := range got {
			if d := math.Abs(float64(got[i] - want[i])); d > 2e-5*(1+math.Abs(float64(want[i]))) {
				t.Fatalf("n %d lane %d: simd %g, scalar %g", n, i, got[i], want[i])
			}
		}
	}
}

func TestGelu(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	x := randF(r, 4099)
	for i := range x {
		x[i] *= 4
	}
	x[0], x[1], x[2] = 0, 40, -40
	want := append([]float32(nil), x...)
	Gelu(want, false)
	got := append([]float32(nil), x...)
	Gelu(got, true)
	for i := range got {
		if d := math.Abs(float64(got[i] - want[i])); d > 2e-6*(1+math.Abs(float64(want[i]))) {
			t.Fatalf("lane %d (x=%g): simd %g, exact %g", i, x[i], got[i], want[i])
		}
	}
}

func TestExp(t *testing.T) {
	r := rand.New(rand.NewPCG(9, 10))
	x := randF(r, 1003)
	for i := range x {
		x[i] = -x[i] * x[i] * 8 // softmax arguments: <= 0
	}
	x[0], x[1], x[2], x[3] = 0, -100, 5, -87
	want := append([]float32(nil), x...)
	Exp(want, false)
	got := append([]float32(nil), x...)
	Exp(got, true)
	for i := range got {
		if d := math.Abs(float64(got[i] - want[i])); d > 3e-7*float64(want[i])+1e-38 {
			t.Fatalf("lane %d (x=%g): simd %g, exact %g", i, x[i], got[i], want[i])
		}
	}
}

func TestHeads(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 8))
	for _, sh := range [][2]int{{8, 32}, {4, 32}, {2, 16}, {3, 48}, {2, 8}} {
		heads, dh := sh[0], sh[1]
		n := heads * dh
		q, k, a := randF(r, n), randF(r, n), randF(r, heads)
		var got, want [8]float32
		DotHeads(got[:heads], q, k, heads, dh, true)
		DotHeads(want[:heads], q, k, heads, dh, false)
		for h := 0; h < heads; h++ {
			if d := math.Abs(float64(got[h] - want[h])); d > 1e-5*float64(dh) {
				t.Fatalf("dot heads %d dh %d head %d: simd %g scalar %g", heads, dh, h, got[h], want[h])
			}
		}
		y0 := randF(r, n)
		y1 := append([]float32(nil), y0...)
		AxpyHeads(y0, q, a, heads, dh, true)
		AxpyHeads(y1, q, a, heads, dh, false)
		for j := range y0 {
			if d := math.Abs(float64(y0[j] - y1[j])); d > 1e-6*(1+math.Abs(float64(y1[j]))) {
				t.Fatalf("axpy heads %d dh %d lane %d: simd %g scalar %g", heads, dh, j, y0[j], y1[j])
			}
		}
	}
}

func BenchmarkGemmFFN(b *testing.B) {
	r := rand.New(rand.NewPCG(1, 2))
	in, out, n := 256, 1024, 22
	p := Pack(randF(r, in*out), randF(r, out), in, out)
	x, y := randF(r, n*in), make([]float32, n*out)
	for b.Loop() {
		Gemm(y, out, x, in, n, p, true)
	}
	b.ReportMetric(float64(2*in*out*n)*float64(b.N)/b.Elapsed().Seconds()/1e9, "GFLOP/s")
}

// BenchmarkGemmCold cycles 48 distinct 1 MB matrices so the weights stream
// from DRAM, as a forward's do in a search: n=1 and 4 are bandwidth-bound,
// n=22 (a typical state's token count) mostly compute-bound.
func BenchmarkGemmCold(b *testing.B) {
	r := rand.New(rand.NewPCG(1, 2))
	in, out := 256, 1024
	var ps []*Packed
	for i := 0; i < 48; i++ {
		ps = append(ps, Pack(randF(r, in*out), randF(r, out), in, out))
	}
	for _, n := range []int{1, 4, 22, 143} {
		b.Run(fmt.Sprintf("n%d", n), func(b *testing.B) {
			x, y := randF(r, n*in), make([]float32, n*out)
			i := 0
			for b.Loop() {
				Gemm(y, out, x, in, n, ps[i%len(ps)], true)
				i++
			}
			b.ReportMetric(float64(2*in*out*n)*float64(b.N)/b.Elapsed().Seconds()/1e9, "GFLOP/s")
			b.ReportMetric(float64(in*out*4)*float64(b.N)/b.Elapsed().Seconds()/1e9, "GB/s")
		})
	}
}
