//go:build amd64

package netsimd

func cpuid(eaxArg, ecxArg uint32) (eax, ebx, ecx, edx uint32)
func xgetbv() (eax, edx uint32)

// hasAVX512 reports AVX-512 F/BW/DQ/VL with the OS saving ZMM state: the
// kernels in kern_amd64.s need exactly that set.
func detectAVX512() bool {
	maxID, _, _, _ := cpuid(0, 0)
	if maxID < 7 {
		return false
	}
	_, _, ecx1, _ := cpuid(1, 0)
	const osxsave, avx = 1 << 27, 1 << 28
	if ecx1&osxsave == 0 || ecx1&avx == 0 {
		return false
	}
	xcr0, _ := xgetbv()
	// XMM, YMM, opmask, ZMM_Hi256, Hi16_ZMM
	if xcr0&0xe6 != 0xe6 {
		return false
	}
	_, ebx7, _, _ := cpuid(7, 0)
	const f, dq, bw, vl = 1 << 16, 1 << 17, 1 << 30, 1 << 31
	return ebx7&(f|dq|bw|vl) == f|dq|bw|vl
}

var hasAVX512 = detectAVX512()

// The assembly kernels (kern_amd64.s). Every pointer is to float32 storage
// the caller sized; n counts elements; outPad is a multiple of 32.

//go:noescape
func gemvTAVX512(wt, x, acc, y *float32, n, outPad int, relu bool)

//go:noescape
func bagAVX512(y, table *float32, feats *feat, n, rowLimit, width int)

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

// bag adds weighted table rows into y. The assembly implements exactly two
// widths (32 and 64) and its fallthrough would treat any other width as 32,
// so anything else takes the width-generic kernel.
func bag(y, table *float32, feats *feat, n, rowLimit, width int) {
	if hasAVX512 && !ForceScalar && (width == 32 || width == 64) {
		bagAVX512(y, table, feats, n, rowLimit, width)
		return
	}
	bagScalar(y, table, feats, n, rowLimit, width)
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
