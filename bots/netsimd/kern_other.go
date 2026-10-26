//go:build !amd64

package netsimd

// hasAVX512 is false off amd64 (the tests name it to skip the assembly arm).
const hasAVX512 = false

func gemvT(wt, x, acc, y *float32, n, outPad int, relu bool) {
	gemvTScalar(wt, x, acc, y, n, outPad, relu)
}
func bag(y, table *float32, feats *feat, n, rowLimit, width int) {
	bagScalar(y, table, feats, n, rowLimit, width)
}
func ln64(y, x, w, b *float32, eps float32) { ln64Scalar(y, x, w, b, eps) }
func dot64(a, b *float32) float32           { return dot64Scalar(a, b) }
func addv64(y, x *float32)                  { addv64Scalar(y, x) }
func poolAcc(sum, mx, c *float32)           { poolAccScalar(sum, mx, c) }

func lnN(y, x, w, b *float32, eps float32, n int) { lnNScalar(y, x, w, b, eps, n) }
func dotN(a, b *float32, n int) float32           { return dotNScalar(a, b, n) }
func addvN(y, x *float32, n int)                  { addvNScalar(y, x, n) }
func poolAccN(sum, mx, c *float32, n int)         { poolAccNScalar(sum, mx, c, n) }

// SIMD reports whether the AVX-512 kernels are in use.
func SIMD() bool { return false }
