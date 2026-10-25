//go:build amd64

#include "textflag.h"

// FMAROW broadcasts one input row's k-th value and accumulates it against
// the panel's four column vectors (Z24..Z27) into four accumulators.
#define FMAROW(ptr, bc, a0, a1, a2, a3) \
	VBROADCASTSS (ptr)(SI*1), bc \
	VFMADD231PS Z24, bc, a0 \
	VFMADD231PS Z25, bc, a1 \
	VFMADD231PS Z26, bc, a2 \
	VFMADD231PS Z27, bc, a3

// STROW adds the bias (Z24..Z27) to one row's accumulators and stores the
// valid columns (masks K1..K4) at DI.
#define STROW(a0, a1, a2, a3) \
	VADDPS Z24, a0, a0 \
	VADDPS Z25, a1, a1 \
	VADDPS Z26, a2, a2 \
	VADDPS Z27, a3, a3 \
	VMOVUPS a0, K1, (DI) \
	VMOVUPS a1, K2, 64(DI) \
	VMOVUPS a2, K3, 128(DI) \
	VMOVUPS a3, K4, 192(DI)

// func gemm6x64AVX512(w, x *float32, xs, k int, y *float32, ys, rows int, mask uint64, bias, pf *float32)
//
// Y[r][0:64] = bias + Σ_{i<k} x[r][i]·w[i*64 : (i+1)*64] for r < rows (1..6):
// 6×64 outputs in 24 zmm accumulators, the panel row in Z24..Z27, one
// broadcast per input row per k. Rows past `rows` alias the last real row's
// pointer (wasted lanes, never stored). pf walks k×256 bytes alongside the
// panel with PREFETCHT1 (the next panel, so its DRAM stream overlaps this
// panel's arithmetic).
TEXT ·gemm6x64AVX512(SB), NOSPLIT, $0-80
	MOVQ pf+72(FP), DI
	MOVQ w+0(FP), AX
	MOVQ x+8(FP), R8
	MOVQ xs+16(FP), DX
	MOVQ k+24(FP), BX
	MOVQ rows+48(FP), CX
	LEAQ (R8)(DX*1), R9
	CMPQ CX, $1
	CMOVQLE R8, R9
	LEAQ (R9)(DX*1), R10
	CMPQ CX, $2
	CMOVQLE R9, R10
	LEAQ (R10)(DX*1), R11
	CMPQ CX, $3
	CMOVQLE R10, R11
	LEAQ (R11)(DX*1), R12
	CMPQ CX, $4
	CMOVQLE R11, R12
	LEAQ (R12)(DX*1), R13
	CMPQ CX, $5
	CMOVQLE R12, R13

	VPXORD Z0, Z0, Z0
	VPXORD Z1, Z1, Z1
	VPXORD Z2, Z2, Z2
	VPXORD Z3, Z3, Z3
	VPXORD Z4, Z4, Z4
	VPXORD Z5, Z5, Z5
	VPXORD Z6, Z6, Z6
	VPXORD Z7, Z7, Z7
	VPXORD Z8, Z8, Z8
	VPXORD Z9, Z9, Z9
	VPXORD Z10, Z10, Z10
	VPXORD Z11, Z11, Z11
	VPXORD Z12, Z12, Z12
	VPXORD Z13, Z13, Z13
	VPXORD Z14, Z14, Z14
	VPXORD Z15, Z15, Z15
	VPXORD Z16, Z16, Z16
	VPXORD Z17, Z17, Z17
	VPXORD Z18, Z18, Z18
	VPXORD Z19, Z19, Z19
	VPXORD Z20, Z20, Z20
	VPXORD Z21, Z21, Z21
	VPXORD Z22, Z22, Z22
	VPXORD Z23, Z23, Z23
	XORQ SI, SI

kloop:
	PREFETCHT1 (DI)
	PREFETCHT1 64(DI)
	PREFETCHT1 128(DI)
	PREFETCHT1 192(DI)
	ADDQ $256, DI
	VMOVUPS (AX), Z24
	VMOVUPS 64(AX), Z25
	VMOVUPS 128(AX), Z26
	VMOVUPS 192(AX), Z27
	FMAROW(R8, Z28, Z0, Z1, Z2, Z3)
	FMAROW(R9, Z29, Z4, Z5, Z6, Z7)
	FMAROW(R10, Z30, Z8, Z9, Z10, Z11)
	FMAROW(R11, Z31, Z12, Z13, Z14, Z15)
	FMAROW(R12, Z28, Z16, Z17, Z18, Z19)
	FMAROW(R13, Z29, Z20, Z21, Z22, Z23)
	ADDQ $256, AX
	ADDQ $4, SI
	DECQ BX
	JNZ  kloop

	MOVQ y+32(FP), DI
	MOVQ ys+40(FP), DX
	MOVQ bias+64(FP), R8
	KMOVQ mask+56(FP), K1
	KSHIFTRQ $16, K1, K2
	KSHIFTRQ $32, K1, K3
	KSHIFTRQ $48, K1, K4
	VMOVUPS (R8), Z24
	VMOVUPS 64(R8), Z25
	VMOVUPS 128(R8), Z26
	VMOVUPS 192(R8), Z27
	STROW(Z0, Z1, Z2, Z3)
	CMPQ CX, $1
	JLE  gdone
	ADDQ DX, DI
	STROW(Z4, Z5, Z6, Z7)
	CMPQ CX, $2
	JLE  gdone
	ADDQ DX, DI
	STROW(Z8, Z9, Z10, Z11)
	CMPQ CX, $3
	JLE  gdone
	ADDQ DX, DI
	STROW(Z12, Z13, Z14, Z15)
	CMPQ CX, $4
	JLE  gdone
	ADDQ DX, DI
	STROW(Z16, Z17, Z18, Z19)
	CMPQ CX, $5
	JLE  gdone
	ADDQ DX, DI
	STROW(Z20, Z21, Z22, Z23)

gdone:
	VZEROUPPER
	RET

// HSUM0 reduces Z0's 16 lanes into X0 lane 0.
#define HSUM0 \
	VEXTRACTF64X4 $1, Z0, Y1 \
	VADDPS Y1, Y0, Y0 \
	VEXTRACTF128 $1, Y0, X1 \
	VADDPS X1, X0, X0 \
	VSHUFPS $0x4E, X0, X0, X1 \
	VADDPS X1, X0, X0 \
	VSHUFPS $0xB1, X0, X0, X1 \
	VADDSS X1, X0, X0

// func lnAVX512(y, x, w, b *float32, n int, eps, invN float32)
//
// y = (x - mean)·(1/sqrt(var + eps))·w + b over n lanes (a multiple of 16),
// fp32 statistics: two passes (mean, then the centred variance).
TEXT ·lnAVX512(SB), NOSPLIT, $0-48
	MOVQ y+0(FP), DI
	MOVQ x+8(FP), SI
	MOVQ w+16(FP), BX
	MOVQ b+24(FP), DX
	MOVQ n+32(FP), CX
	SHLQ $2, CX
	VBROADCASTSS invN+44(FP), Z5

	VPXORD Z0, Z0, Z0
	VPXORD Z2, Z2, Z2
	XORQ AX, AX
	TESTQ $64, CX
	JZ   lnsum
	VMOVUPS (SI), Z0
	MOVQ $64, AX
	CMPQ AX, CX
	JGE  lnsumd
lnsum:
	VADDPS (SI)(AX*1), Z0, Z0
	VADDPS 64(SI)(AX*1), Z2, Z2
	ADDQ $128, AX
	CMPQ AX, CX
	JL   lnsum
lnsumd:
	VADDPS Z2, Z0, Z0
	HSUM0
	VMULSS X5, X0, X0
	VBROADCASTSS X0, Z6          // mean

	VPXORD Z0, Z0, Z0
	VPXORD Z2, Z2, Z2
	XORQ AX, AX
	TESTQ $64, CX
	JZ   lnvar
	VMOVUPS (SI), Z3
	VSUBPS Z6, Z3, Z3
	VMULPS Z3, Z3, Z0
	MOVQ $64, AX
	CMPQ AX, CX
	JGE  lnvard
lnvar:
	VMOVUPS (SI)(AX*1), Z3
	VMOVUPS 64(SI)(AX*1), Z4
	VSUBPS Z6, Z3, Z3
	VSUBPS Z6, Z4, Z4
	VFMADD231PS Z3, Z3, Z0
	VFMADD231PS Z4, Z4, Z2
	ADDQ $128, AX
	CMPQ AX, CX
	JL   lnvar
lnvard:
	VADDPS Z2, Z0, Z0
	HSUM0
	VMULSS X5, X0, X0
	VADDSS eps+40(FP), X0, X0
	VSQRTSS X0, X0, X0
	MOVL $0x3f800000, R8         // 1.0
	MOVD R8, X1
	VDIVSS X0, X1, X0
	VBROADCASTSS X0, Z7          // inv

	XORQ AX, AX
lnout:
	VMOVUPS (SI)(AX*1), Z3
	VSUBPS Z6, Z3, Z3
	VMULPS Z7, Z3, Z3
	VMULPS (BX)(AX*1), Z3, Z3
	VADDPS (DX)(AX*1), Z3, Z3
	VMOVUPS Z3, (DI)(AX*1)
	ADDQ $64, AX
	CMPQ AX, CX
	JL   lnout
	VZEROUPPER
	RET

// func geluAVX512(x *float32, n int, c *float32)
//
// x = 0.5·x·(1 + erf(x/√2)) in place over n lanes (a multiple of 16, > 0),
// erf as Eigen's rational approximation on the clamp [-4, 4]. c is
// simdk.geluConst.
TEXT ·geluAVX512(SB), NOSPLIT, $0-24
	MOVQ x+0(FP), DI
	MOVQ n+8(FP), SI
	MOVQ c+16(FP), CX
	VBROADCASTSS 0(CX), Z16      // 1/√2
	VBROADCASTSS 4(CX), Z17      // 4
	VBROADCASTSS 8(CX), Z18      // -4
	VBROADCASTSS 12(CX), Z19     // 0.5
	VBROADCASTSS 16(CX), Z20     // a13
	VBROADCASTSS 20(CX), Z21
	VBROADCASTSS 24(CX), Z22
	VBROADCASTSS 28(CX), Z23
	VBROADCASTSS 32(CX), Z24
	VBROADCASTSS 36(CX), Z25
	VBROADCASTSS 40(CX), Z26     // a1
	VBROADCASTSS 44(CX), Z27     // b8
	VBROADCASTSS 48(CX), Z28
	VBROADCASTSS 52(CX), Z29
	VBROADCASTSS 56(CX), Z30
	VBROADCASTSS 60(CX), Z31     // b0

gloop:
	VMOVUPS (DI), Z0
	VMULPS Z16, Z0, Z1
	VMINPS Z17, Z1, Z1
	VMAXPS Z18, Z1, Z1
	VMULPS Z1, Z1, Z2
	VMOVAPS Z20, Z3
	VFMADD213PS Z21, Z2, Z3
	VFMADD213PS Z22, Z2, Z3
	VFMADD213PS Z23, Z2, Z3
	VFMADD213PS Z24, Z2, Z3
	VFMADD213PS Z25, Z2, Z3
	VFMADD213PS Z26, Z2, Z3
	VMULPS Z1, Z3, Z3
	VMOVAPS Z27, Z4
	VFMADD213PS Z28, Z2, Z4
	VFMADD213PS Z29, Z2, Z4
	VFMADD213PS Z30, Z2, Z4
	VFMADD213PS Z31, Z2, Z4
	VDIVPS Z4, Z3, Z3
	VMULPS Z19, Z0, Z5
	VFMADD213PS Z5, Z5, Z3
	VMOVUPS Z3, (DI)
	ADDQ $64, DI
	SUBQ $16, SI
	JNZ  gloop
	VZEROUPPER
	RET

// func dotHeadsAVX512(q, k *float32, heads, dh int, out *float32)
//
// out[h] = Σ_j q[h*dh+j]·k[h*dh+j] for each head h; dh a multiple of 16.
TEXT ·dotHeadsAVX512(SB), NOSPLIT, $0-40
	MOVQ q+0(FP), SI
	MOVQ k+8(FP), DI
	MOVQ heads+16(FP), CX
	MOVQ dh+24(FP), DX
	MOVQ out+32(FP), R8
dhh:
	VPXORD Z0, Z0, Z0
	MOVQ DX, BX
dhj:
	VMOVUPS (SI), Z2
	VFMADD231PS (DI), Z2, Z0
	ADDQ $64, SI
	ADDQ $64, DI
	SUBQ $16, BX
	JNZ  dhj
	HSUM0
	VMOVSS X0, (R8)
	ADDQ $4, R8
	DECQ CX
	JNZ  dhh
	VZEROUPPER
	RET

// func axpyHeadsAVX512(y, x, a *float32, heads, dh int)
//
// y[h*dh+j] += a[h]·x[h*dh+j] for each head h; dh a multiple of 16.
TEXT ·axpyHeadsAVX512(SB), NOSPLIT, $0-40
	MOVQ y+0(FP), DI
	MOVQ x+8(FP), SI
	MOVQ a+16(FP), R8
	MOVQ heads+24(FP), CX
	MOVQ dh+32(FP), DX
axh:
	VBROADCASTSS (R8), Z1
	MOVQ DX, BX
axj:
	VMOVUPS (DI), Z0
	VFMADD231PS (SI), Z1, Z0
	VMOVUPS Z0, (DI)
	ADDQ $64, SI
	ADDQ $64, DI
	SUBQ $16, BX
	JNZ  axj
	ADDQ $4, R8
	DECQ CX
	JNZ  axh
	VZEROUPPER
	RET

// func expAVX512(x *float32, n int, c *float32)
//
// x = exp(x) in place over n lanes (a multiple of 16, > 0): Cephes expf's
// range reduction (n = round(x·log2e), r = x - n·ln2 in two parts), its
// degree-5 polynomial, and VSCALEFPS for the 2^n. c is simdk.expConst.
TEXT ·expAVX512(SB), NOSPLIT, $0-24
	MOVQ x+0(FP), DI
	MOVQ n+8(FP), SI
	MOVQ c+16(FP), CX
	VBROADCASTSS 0(CX), Z16      // hi clamp
	VBROADCASTSS 4(CX), Z17      // lo clamp
	VBROADCASTSS 8(CX), Z18      // log2e
	VBROADCASTSS 12(CX), Z19     // ln2 hi
	VBROADCASTSS 16(CX), Z20     // ln2 lo
	VBROADCASTSS 20(CX), Z21     // p0
	VBROADCASTSS 24(CX), Z22
	VBROADCASTSS 28(CX), Z23
	VBROADCASTSS 32(CX), Z24
	VBROADCASTSS 36(CX), Z25
	VBROADCASTSS 40(CX), Z26     // p5
	VBROADCASTSS 44(CX), Z27     // 1
eloop:
	VMOVUPS (DI), Z0
	VMINPS Z16, Z0, Z0
	VMAXPS Z17, Z0, Z0
	VMULPS Z18, Z0, Z1
	VRNDSCALEPS $0, Z1, Z1       // n = round(x·log2e)
	VFNMADD231PS Z19, Z1, Z0     // r = x - n·ln2hi
	VFNMADD231PS Z20, Z1, Z0     // r -= n·ln2lo
	VMOVAPS Z21, Z2
	VFMADD213PS Z22, Z0, Z2
	VFMADD213PS Z23, Z0, Z2
	VFMADD213PS Z24, Z0, Z2
	VFMADD213PS Z25, Z0, Z2
	VFMADD213PS Z26, Z0, Z2      // p(r)
	VMULPS Z0, Z0, Z3            // r²
	VADDPS Z27, Z0, Z4           // 1 + r
	VFMADD231PS Z3, Z2, Z4       // 1 + r + p·r²
	VSCALEFPS Z1, Z4, Z4         // ·2^n
	VMOVUPS Z4, (DI)
	ADDQ $64, DI
	SUBQ $16, SI
	JNZ  eloop
	VZEROUPPER
	RET

// func gemm4x64AVX512(w, x *float32, xs, k int, y *float32, ys, rows int, mask uint64, bias, pf *float32)
//
// gemm6x64AVX512 for rows <= 4: 16 accumulators, the same per-row
// arithmetic in the same order (so a row's result does not depend on which
// kernel ran it), two fewer broadcast-and-FMA rows per k.
TEXT ·gemm4x64AVX512(SB), NOSPLIT, $0-80
	MOVQ pf+72(FP), DI
	MOVQ w+0(FP), AX
	MOVQ x+8(FP), R8
	MOVQ xs+16(FP), DX
	MOVQ k+24(FP), BX
	MOVQ rows+48(FP), CX
	LEAQ (R8)(DX*1), R9
	CMPQ CX, $1
	CMOVQLE R8, R9
	LEAQ (R9)(DX*1), R10
	CMPQ CX, $2
	CMOVQLE R9, R10
	LEAQ (R10)(DX*1), R11
	CMPQ CX, $3
	CMOVQLE R10, R11

	VPXORD Z0, Z0, Z0
	VPXORD Z1, Z1, Z1
	VPXORD Z2, Z2, Z2
	VPXORD Z3, Z3, Z3
	VPXORD Z4, Z4, Z4
	VPXORD Z5, Z5, Z5
	VPXORD Z6, Z6, Z6
	VPXORD Z7, Z7, Z7
	VPXORD Z8, Z8, Z8
	VPXORD Z9, Z9, Z9
	VPXORD Z10, Z10, Z10
	VPXORD Z11, Z11, Z11
	VPXORD Z12, Z12, Z12
	VPXORD Z13, Z13, Z13
	VPXORD Z14, Z14, Z14
	VPXORD Z15, Z15, Z15
	XORQ SI, SI

k4loop:
	PREFETCHT1 (DI)
	PREFETCHT1 64(DI)
	PREFETCHT1 128(DI)
	PREFETCHT1 192(DI)
	ADDQ $256, DI
	VMOVUPS (AX), Z24
	VMOVUPS 64(AX), Z25
	VMOVUPS 128(AX), Z26
	VMOVUPS 192(AX), Z27
	FMAROW(R8, Z28, Z0, Z1, Z2, Z3)
	FMAROW(R9, Z29, Z4, Z5, Z6, Z7)
	FMAROW(R10, Z30, Z8, Z9, Z10, Z11)
	FMAROW(R11, Z31, Z12, Z13, Z14, Z15)
	ADDQ $256, AX
	ADDQ $4, SI
	DECQ BX
	JNZ  k4loop

	MOVQ y+32(FP), DI
	MOVQ ys+40(FP), DX
	MOVQ bias+64(FP), R8
	KMOVQ mask+56(FP), K1
	KSHIFTRQ $16, K1, K2
	KSHIFTRQ $32, K1, K3
	KSHIFTRQ $48, K1, K4
	VMOVUPS (R8), Z24
	VMOVUPS 64(R8), Z25
	VMOVUPS 128(R8), Z26
	VMOVUPS 192(R8), Z27
	STROW(Z0, Z1, Z2, Z3)
	CMPQ CX, $1
	JLE  g4done
	ADDQ DX, DI
	STROW(Z4, Z5, Z6, Z7)
	CMPQ CX, $2
	JLE  g4done
	ADDQ DX, DI
	STROW(Z8, Z9, Z10, Z11)
	CMPQ CX, $3
	JLE  g4done
	ADDQ DX, DI
	STROW(Z12, Z13, Z14, Z15)

g4done:
	VZEROUPPER
	RET
