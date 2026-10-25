//go:build !amd64

package simdk

// AVX512 is false off amd64: every kernel takes its scalar twin.
const AVX512 = false

func gemmAVX512(y []float32, ys int, x []float32, xs int, n int, p *Packed) {
	panic("simdk: no AVX-512 kernels on this architecture")
}

func lnAVX512(y, x, w, b *float32, n int, eps, invN float32) {
	panic("simdk: no AVX-512 kernels on this architecture")
}

func geluAVX512(x *float32, n int, c *float32) {
	panic("simdk: no AVX-512 kernels on this architecture")
}

func dotHeadsAVX512(q, k *float32, heads, dh int, out *float32) {
	panic("simdk: no AVX-512 kernels on this architecture")
}

func axpyHeadsAVX512(y, x, a *float32, heads, dh int) {
	panic("simdk: no AVX-512 kernels on this architecture")
}

func expAVX512(x *float32, n int, c *float32) {
	panic("simdk: no AVX-512 kernels on this architecture")
}
