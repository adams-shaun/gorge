package netsimd

import (
	"math"
	"unsafe"
)

// ForceScalar routes every kernel through the Go fallback (tests compare it
// against the assembly; a benchmark measures the gap).
var ForceScalar bool

// feat mirrors policynet.Feature's layout (Row uint16 at 0, Value float32 at
// 4, 8 bytes): the bag kernels walk the caller's []policynet.Feature in
// place. layout_test.go pins it.
type feat struct {
	Row   uint16
	Value float32
}

func f32s(p *float32, n int) []float32 { return unsafe.Slice(p, n) }

// gemvTScalar: y[:outPad] = acc[:outPad] + Σ_{i<n} x[i]·wt[i*outPad : (i+1)*outPad],
// then relu when asked. The order of summation matches the AVX-512 kernel's
// per-lane accumulation only in structure (one running sum per output,
// over i), not in its even/odd split; both are "one fp32 sum per lane".
func gemvTScalar(wt, x, acc, y *float32, n, outPad int, relu bool) {
	W := f32s(wt, n*outPad)
	X := f32s(x, n)
	A := f32s(acc, outPad)
	Y := f32s(y, outPad)
	for o := 0; o < outPad; o++ {
		// Two running sums (even and odd i) summed at the end, as the
		// assembly does, so the two paths agree bit for bit on every
		// shape where FMA rounding is the only difference.
		var s0, s1 float32
		for i := 0; i+2 <= n; i += 2 {
			s0 = fma32(X[i], W[i*outPad+o], s0)
			s1 = fma32(X[i+1], W[(i+1)*outPad+o], s1)
		}
		if n&1 == 1 {
			s0 = fma32(X[n-1], W[(n-1)*outPad+o], s0)
		}
		v := A[o] + (s0 + s1)
		if relu && v < 0 {
			v = 0
		}
		Y[o] = v
	}
}

// fma32 is a fused multiply-add rounded once, as VFMADD231PS rounds.
func fma32(a, b, c float32) float32 { return float32(math.FMA(float64(a), float64(b), float64(c))) }

// bagScalar: y[:width] += Σ feats[i].Value · table[row*width : (row+1)*width]
// over rows < rowLimit.
func bagScalar(y, table *float32, feats *feat, n, rowLimit, width int) {
	Y := f32s(y, width)
	F := unsafe.Slice(feats, n)
	T := f32s(table, rowLimit*width)
	for _, f := range F {
		r := int(f.Row)
		if r >= rowLimit {
			continue
		}
		row := T[r*width : (r+1)*width]
		for j := range Y {
			Y[j] = fma32(f.Value, row[j], Y[j])
		}
	}
}

// hsum16 is the assembly's reduction of 16 lane sums: 16→8→4, then
// (l0+l2)+(l1+l3).
func hsum16(s *[16]float32) float32 {
	for w := 8; w >= 1; w /= 2 {
		for j := 0; j < w; j++ {
			s[j] += s[j+w]
		}
	}
	return s[0]
}

// ln64Scalar: y = LN(x) over 64 lanes in fp32 (gonet accumulates in float64;
// the difference is ~1e-7 relative on a 64-wide vector). The lane sums are
// ((q0+q1)+(q2+q3)) over the four 16-lane quarters, then hsum16, as the
// assembly reduces.
func ln64Scalar(y, x, w, b *float32, eps float32) {
	X := f32s(x, 64)
	Y := f32s(y, 64)
	Wv := f32s(w, 64)
	B := f32s(b, 64)
	var s [16]float32
	for j := 0; j < 16; j++ {
		s[j] = (X[j] + X[16+j]) + (X[32+j] + X[48+j])
	}
	m := hsum16(&s) * (1.0 / 64)
	var d [64]float32
	for i, v := range X {
		d[i] = v - m
	}
	for j := 0; j < 16; j++ {
		s[j] = (float32(d[j]*d[j]) + float32(d[16+j]*d[16+j])) + (float32(d[32+j]*d[32+j]) + float32(d[48+j]*d[48+j]))
	}
	q := hsum16(&s) * (1.0 / 64)
	inv := float32(1) / float32(math.Sqrt(float64(q+eps)))
	for i := range Y {
		Y[i] = float32(float32(d[i]*inv)*Wv[i]) + B[i]
	}
}

func dot64Scalar(a, b *float32) float32 {
	A := f32s(a, 64)
	B := f32s(b, 64)
	var s [16]float32
	for i := 0; i < 64; i += 16 {
		for j := 0; j < 16; j++ {
			s[j] = fma32(A[i+j], B[i+j], s[j])
		}
	}
	return hsum16(&s)
}

func addv64Scalar(y, x *float32) {
	Y := f32s(y, 64)
	X := f32s(x, 64)
	for i := range Y {
		Y[i] += X[i]
	}
}

// poolAccScalar: sum[:32] += 0.25·c, mx[:32] = max(mx, c).
func poolAccScalar(sum, mx, c *float32) {
	S := f32s(sum, 32)
	M := f32s(mx, 32)
	C := f32s(c, 32)
	for i := range C {
		S[i] = fma32(0.25, C[i], S[i])
		if C[i] > M[i] {
			M[i] = C[i]
		}
	}
}

// poolAccN: sum[:n] += 0.25·c, mx[:n] = max(mx, c), over a width-generic n.
func poolAccN(sum, mx, c *float32, n int) {
	S := f32s(sum, n)
	M := f32s(mx, n)
	C := f32s(c, n)
	for i := range C {
		S[i] = fma32(0.25, C[i], S[i])
		if C[i] > M[i] {
			M[i] = C[i]
		}
	}
}

// poolAccW pools a cd-wide card vector: the 32-lane assembly kernel is the
// fast path, every other width takes the generic kernel.
func poolAccW(sum, mx, c *float32, cd int) {
	if cd == 32 {
		poolAcc(sum, mx, c)
		return
	}
	poolAccN(sum, mx, c, cd)
}

// Width-generic variants. The 64-lane kernels above are the fast path, tuned
// against the assembly; these serve every other width (a wide MLP's d=128, a
// GNN's 128/384) and the fast path's own scalar reference. They accumulate in
// float64 so a wide vector's layernorm does not drift with length; the 64-lane
// kernels keep their fp32 quarter sums because the assembly reduces that way
// and kern_test.go compares them bit-for-bit.
//
// lnN: y = ((x - mean(x)) / sqrt(var(x) + eps)) * w + b over n lanes.
func lnN(y, x, w, b *float32, eps float32, n int) {
	X := f32s(x, n)
	Y := f32s(y, n)
	Wv := f32s(w, n)
	Bv := f32s(b, n)
	var sum float64
	for _, v := range X {
		sum += float64(v)
	}
	m := sum / float64(n)
	var sq float64
	for _, v := range X {
		d := float64(v) - m
		sq += d * d
	}
	inv := 1 / math.Sqrt(sq/float64(n)+float64(eps))
	for i, v := range X {
		Y[i] = float32((float64(v)-m)*inv*float64(Wv[i])) + Bv[i]
	}
}

// dotN: the sum of a[i]*b[i] over n lanes, accumulated in float64.
func dotN(a, b *float32, n int) float32 {
	A := f32s(a, n)
	B := f32s(b, n)
	var s float64
	for i := range A {
		s += float64(A[i]) * float64(B[i])
	}
	return float32(s)
}

// addvN: y[i] += x[i] over n lanes.
func addvN(y, x *float32, n int) {
	Y := f32s(y, n)
	X := f32s(x, n)
	for i := range Y {
		Y[i] += X[i]
	}
}
