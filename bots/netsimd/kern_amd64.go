//go:build amd64

package netsimd

import "github.com/adams-shaun/gorge/bots/internal/simdk"

// hasAVX512 is the shared detection (bots/internal/simdk): AVX-512
// F/BW/DQ/VL with the OS saving ZMM state, exactly what kern_amd64.s needs.
var hasAVX512 = simdk.AVX512

// The assembly kernels (kern_amd64.s). Every pointer is to float32 storage
// the caller sized; n counts elements; outPad is a multiple of 32.

//go:noescape
func gemvTAVX512(wt, x, acc, y *float32, n, outPad int, relu bool)

//go:noescape
func bagAVX512(y, table *float32, feats *feat, n, rowLimit, width int)

// bagNAVX512 is bagAVX512 for any width that is a multiple of 16: the
// columns in blocks of 128 lanes (eight zmm), then 16-lane blocks, the
// features walked once per block.
//
//go:noescape
func bagNAVX512(y, table *float32, feats *feat, n, rowLimit, width int)

//go:noescape
func lnNAVX512(y, x, w, b *float32, n int, eps, invN float32)

//go:noescape
func dotNAVX512(a, b *float32, n int) float32

//go:noescape
func addvNAVX512(y, x *float32, n int)

//go:noescape
func poolAccNAVX512(sum, mx, c *float32, n int)

//go:noescape
func ln64AVX512(y, x, w, b *float32, eps float32)

//go:noescape
func dot64AVX512(a, b *float32) float32

//go:noescape
func addv64AVX512(y, x *float32)

//go:noescape
func poolAccAVX512(sum, mx, c *float32)

func gemvT(wt, x, acc, y *float32, n, outPad int, relu bool) {
	if hasAVX512 && !ForceScalar {
		gemvTAVX512(wt, x, acc, y, n, outPad, relu)
		return
	}
	gemvTScalar(wt, x, acc, y, n, outPad, relu)
}

// bag adds weighted table rows into y. bagAVX512 implements exactly two
// widths (32 and 64; its fallthrough would treat any other width as 32);
// bagNAVX512 serves every other multiple of 16, and anything else takes the
// scalar kernel. Every lane is an independent FMA chain in feature order, so
// all three are bit-identical.
func bag(y, table *float32, feats *feat, n, rowLimit, width int) {
	if hasAVX512 && !ForceScalar {
		switch {
		case width == 32 || width == 64:
			bagAVX512(y, table, feats, n, rowLimit, width)
			return
		case width >= 16 && width%16 == 0:
			bagNAVX512(y, table, feats, n, rowLimit, width)
			return
		}
	}
	bagScalar(y, table, feats, n, rowLimit, width)
}

// vecN reports whether an n-lane vector kernel can take the assembly.
func vecN(n int) bool { return hasAVX512 && !ForceScalar && n >= 16 && n%16 == 0 }

// lnN is the width-generic layernorm (lnNScalar's contract).
func lnN(y, x, w, b *float32, eps float32, n int) {
	if vecN(n) {
		lnNAVX512(y, x, w, b, n, eps, float32(1)/float32(n))
		return
	}
	lnNScalar(y, x, w, b, eps, n)
}

// dotN is the width-generic dot product (dotNScalar's contract).
func dotN(a, b *float32, n int) float32 {
	if vecN(n) {
		return dotNAVX512(a, b, n)
	}
	return dotNScalar(a, b, n)
}

// addvN is the width-generic vector add.
func addvN(y, x *float32, n int) {
	if vecN(n) {
		addvNAVX512(y, x, n)
		return
	}
	addvNScalar(y, x, n)
}

// poolAccN is the width-generic pooling step.
func poolAccN(sum, mx, c *float32, n int) {
	if vecN(n) {
		poolAccNAVX512(sum, mx, c, n)
		return
	}
	poolAccNScalar(sum, mx, c, n)
}

func ln64(y, x, w, b *float32, eps float32) {
	if hasAVX512 && !ForceScalar {
		ln64AVX512(y, x, w, b, eps)
		return
	}
	ln64Scalar(y, x, w, b, eps)
}

func dot64(a, b *float32) float32 {
	if hasAVX512 && !ForceScalar {
		return dot64AVX512(a, b)
	}
	return dot64Scalar(a, b)
}

func addv64(y, x *float32) {
	if hasAVX512 && !ForceScalar {
		addv64AVX512(y, x)
		return
	}
	addv64Scalar(y, x)
}

func poolAcc(sum, mx, c *float32) {
	if hasAVX512 && !ForceScalar {
		poolAccAVX512(sum, mx, c)
		return
	}
	poolAccScalar(sum, mx, c)
}

// SIMD reports whether the AVX-512 kernels are in use.
func SIMD() bool { return hasAVX512 && !ForceScalar }
