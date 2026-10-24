//go:build amd64

package simdk

func cpuid(eaxArg, ecxArg uint32) (eax, ebx, ecx, edx uint32)
func xgetbv() (eax, edx uint32)

// detectAVX512 reports AVX-512 F/BW/DQ/VL with the OS saving ZMM state: the
// kernels here and in bots/netsimd need exactly that set.
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

// AVX512 reports whether this CPU runs the AVX-512 kernels. Callers treat it
// as read-only.
var AVX512 = detectAVX512()

// gemm6x64AVX512 is the microkernel: rows (1..MR) input rows starting at x,
// stride xs bytes, against one packed panel w (k×NR), plus the panel's bias;
// the result's rows go to y (stride ys bytes), columns masked by mask. Rows
// past `rows` repeat the last real row and are never stored.
//
//go:noescape
func gemm6x64AVX512(w, x *float32, xs, k int, y *float32, ys, rows int, mask uint64, bias, pf *float32)

// gemm4x64AVX512 is gemm6x64AVX512 for rows <= 4 (16 accumulators).
//
//go:noescape
func gemm4x64AVX512(w, x *float32, xs, k int, y *float32, ys, rows int, mask uint64, bias, pf *float32)

//go:noescape
func lnAVX512(y, x, w, b *float32, n int, eps, invN float32)

//go:noescape
func geluAVX512(x *float32, n int, c *float32)

//go:noescape
func expAVX512(x *float32, n int, c *float32)

//go:noescape
func dotHeadsAVX512(q, k *float32, heads, dh int, out *float32)

//go:noescape
func axpyHeadsAVX512(y, x, a *float32, heads, dh int)

// gemmAVX512 walks the panels outermost, so each panel streams from memory
// once and stays in L2 while every row block reuses it.
func gemmAVX512(y []float32, ys int, x []float32, xs int, n int, p *Packed) {
	in := p.In
	for pan := 0; pan < p.Panels; pan++ {
		w := &p.W[pan*in*NR]
		b := &p.B[pan*NR]
		mask := p.colMask(pan)
		pf := w // the last panel prefetches itself (already cached)
		if pan+1 < p.Panels {
			pf = &p.W[(pan+1)*in*NR]
		}
		for r := 0; r < n; r += MR {
			rows := min(MR, n-r)
			if rows <= 4 {
				gemm4x64AVX512(w, &x[r*xs], xs*4, in, &y[r*ys+pan*NR], ys*4, rows, mask, b, pf)
				continue
			}
			gemm6x64AVX512(w, &x[r*xs], xs*4, in, &y[r*ys+pan*NR], ys*4, rows, mask, b, pf)
		}
	}
}
