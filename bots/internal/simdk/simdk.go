// Package simdk holds the width-generic AVX-512 kernels the bots' network
// forwards share: CPU feature detection, a register-blocked fp32 GEMM over
// pre-packed weight panels, LayerNorm and GELU. Every kernel has a scalar Go
// twin used on other CPUs (and on request), so callers keep one code path.
//
// The kernels allocate nothing and read no clock.
package simdk

import "math"

// NR is the column width of a packed weight panel: four zmm registers.
const NR = 64

// MR is the number of input rows the GEMM microkernel holds in registers.
const MR = 6

// Packed is a Linear layer's weight (PyTorch's out×in, row-major) laid out
// for the GEMM: the outputs in panels of NR columns, each panel in×NR floats
// contiguous (row k of a panel is the k-th input's weights for those NR
// outputs), the last panel zero-padded. B is the bias padded to Panels×NR
// (zeros without one).
type Packed struct {
	In, Out, Panels int
	W               []float32
	B               []float32
}

// Pack lays out w (out×in, row-major) and b (out, or nil) as panels.
func Pack(w, b []float32, in, out int) *Packed {
	np := (out + NR - 1) / NR
	p := &Packed{In: in, Out: out, Panels: np, W: make([]float32, np*in*NR), B: make([]float32, np*NR)}
	for o := 0; o < out; o++ {
		pan, col := o/NR, o%NR
		base := pan * in * NR
		row := w[o*in : (o+1)*in]
		for k, v := range row {
			p.W[base+k*NR+col] = v
		}
	}
	copy(p.B, b)
	return p
}

// colMask is the store mask of panel pan: one bit per valid column.
func (p *Packed) colMask(pan int) uint64 {
	c := p.Out - pan*NR
	if c >= NR {
		return ^uint64(0)
	}
	return 1<<uint(c) - 1
}

// Gemm computes Y = X·Wᵀ + b for n input rows: row r of X starts at
// x[r*xs] (In floats), row r of Y at y[r*ys] (Out floats). simd selects the
// AVX-512 kernel when the CPU has it; false forces the scalar twin.
func Gemm(y []float32, ys int, x []float32, xs int, n int, p *Packed, simd bool) {
	if n <= 0 {
		return
	}
	in := p.In
	_ = x[(n-1)*xs+in-1]
	_ = y[(n-1)*ys+p.Out-1]
	if simd && AVX512 {
		gemmAVX512(y, ys, x, xs, n, p)
		return
	}
	for r := 0; r < n; r++ {
		xr := x[r*xs : r*xs+in]
		var acc [NR]float32
		for pan := 0; pan < p.Panels; pan++ {
			copy(acc[:], p.B[pan*NR:(pan+1)*NR])
			w := p.W[pan*in*NR : (pan+1)*in*NR]
			for k, xv := range xr {
				wk := w[k*NR : (k+1)*NR : (k+1)*NR]
				for j := range acc {
					acc[j] += xv * wk[j]
				}
			}
			cols := min(NR, p.Out-pan*NR)
			copy(y[r*ys+pan*NR:r*ys+pan*NR+cols], acc[:cols])
		}
	}
}

// LayerNorm writes (x - mean)·rsqrt(var + eps)·w + b into y (y may be x),
// statistics in fp32 on the SIMD path and float64 on the scalar one. The
// SIMD path needs len(x) to be a multiple of 16; any other width falls back.
func LayerNorm(y, x, w, b []float32, eps float64, simd bool) {
	n := len(x)
	y, w, b = y[:n], w[:n], b[:n]
	if simd && AVX512 && n >= 16 && n%16 == 0 {
		lnAVX512(&y[0], &x[0], &w[0], &b[0], n, float32(eps), 1/float32(n))
		return
	}
	var m float64
	for _, v := range x {
		m += float64(v)
	}
	m /= float64(n)
	var q float64
	for _, v := range x {
		dv := float64(v) - m
		q += dv * dv
	}
	q /= float64(n)
	inv := 1 / math.Sqrt(q+eps)
	for i, v := range x {
		y[i] = float32((float64(v)-m)*inv)*w[i] + b[i]
	}
}

// Gelu applies F.gelu (the erf form) in place. The scalar path calls
// math.Erf; the SIMD path uses a rational erf approximation (Eigen's
// generic_fast_erf_float, |error| < 5e-7) on every lane, the ragged tail
// through a padded block, so a lane's result never depends on its position.
func Gelu(x []float32, simd bool) {
	if simd && AVX512 && len(x) > 0 {
		i := len(x) &^ 15
		if i > 0 {
			geluAVX512(&x[0], i, &geluConst[0])
		}
		if i < len(x) {
			var t [16]float32
			m := copy(t[:], x[i:])
			geluAVX512(&t[0], 16, &geluConst[0])
			copy(x[i:], t[:m])
		}
		return
	}
	for i := range x {
		v := float64(x[i])
		x[i] = float32(0.5 * v * (1 + math.Erf(v/math.Sqrt2)))
	}
}

// Exp replaces x with exp(x) in place (softmax numerators: callers shift by
// the max first). The scalar path calls math.Exp; the SIMD path is Cephes
// expf (relative error ~1e-7) on every lane, the ragged tail through a
// padded block, so a lane's result never depends on its position.
func Exp(x []float32, simd bool) {
	if simd && AVX512 && len(x) > 0 {
		i := len(x) &^ 15
		if i > 0 {
			expAVX512(&x[0], i, &expConst[0])
		}
		if i < len(x) {
			var t [16]float32
			m := copy(t[:], x[i:])
			expAVX512(&t[0], 16, &expConst[0])
			copy(x[i:], t[:m])
		}
		return
	}
	for i := range x {
		x[i] = float32(math.Exp(float64(x[i])))
	}
}

// expConst is the exp kernel's constant table: the input clamp, log2(e),
// ln 2 split hi/lo, Cephes expf's polynomial p0..p5, and 1.
var expConst = [12]float32{
	88.3762626647949, -88.3762626647949, 1.44269504088896341, 0.693359375, -2.12194440e-4,
	1.9875691500e-4, 1.3981999507e-3, 8.3334519073e-3, 4.1665795894e-2, 1.6666665459e-1, 5.0000001201e-1,
	1,
}

// DotHeads writes, for each of heads blocks of dh lanes, the block's dot
// product of q and k into out[h] (an attention logit per head). The SIMD
// path needs dh to be a multiple of 16.
func DotHeads(out, q, k []float32, heads, dh int, simd bool) {
	n := heads * dh
	q, k, out = q[:n], k[:n], out[:heads]
	if simd && AVX512 && dh%16 == 0 && heads > 0 {
		dotHeadsAVX512(&q[0], &k[0], heads, dh, &out[0])
		return
	}
	for h := range out {
		var s float32
		for j := h * dh; j < (h+1)*dh; j++ {
			s += q[j] * k[j]
		}
		out[h] = s
	}
}

// AxpyHeads adds a[h]·x to y over each of heads blocks of dh lanes (one
// value row's contribution to every head's attention output). The SIMD
// path needs dh to be a multiple of 16.
func AxpyHeads(y, x, a []float32, heads, dh int, simd bool) {
	n := heads * dh
	y, x, a = y[:n], x[:n], a[:heads]
	if simd && AVX512 && dh%16 == 0 && heads > 0 {
		axpyHeadsAVX512(&y[0], &x[0], &a[0], heads, dh)
		return
	}
	for h, av := range a {
		for j := h * dh; j < (h+1)*dh; j++ {
			y[j] += av * x[j]
		}
	}
}

// geluConst is the GELU kernel's constant table (kern_amd64.s reads it by
// offset): 1/√2, the clamp ±4, 0.5, the odd numerator coefficients a13..a1
// and the even denominator coefficients b8..b0.
var geluConst = [16]float32{
	1 / math.Sqrt2, 4, -4, 0.5,
	-2.72614225801306e-10, 2.77068142495902e-08, -2.10102402082508e-06, -5.69250639462346e-05,
	-7.34990630326855e-04, -2.95459980854025e-03, -1.60960333262415e-02,
	-1.45660718464996e-05, -2.13374055278905e-04, -1.68282697438203e-03, -7.37332916720468e-03, -1.42647390514189e-02,
}
