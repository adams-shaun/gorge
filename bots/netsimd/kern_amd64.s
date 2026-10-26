//go:build amd64

#include "textflag.h"

// Every kernel is a whole-layer call: one entry, the full loop inside, so the
// no-inline cost of an assembly function is paid once per layer, not per
// lane. Scalars in [...]Scalar (kern_generic.go) spell the same arithmetic in
// the same order; a test holds the two bit-identical.

// func gemvTAVX512(wt, x, acc, y *float32, n, outPad int, relu bool)
//
// y[:outPad] = acc[:outPad] + Σ_{i<n} x[i]·wt[i*outPad : (i+1)*outPad]
// (then max(·,0) when relu). outPad is a multiple of 32. Columns are walked
// in blocks of 64 lanes (four zmm) with the i loop unrolled by two into even
// and odd accumulator sets — eight independent FMA chains, enough to keep
// both of Zen 5's 512-bit FMA pipes busy — then a 32-lane block for the
// remainder.
TEXT ·gemvTAVX512(SB), NOSPLIT, $0-49
	MOVQ wt+0(FP), AX
	MOVQ x+8(FP), BX
	MOVQ acc+16(FP), CX
	MOVQ y+24(FP), DX
	MOVQ n+32(FP), SI
	MOVQ outPad+40(FP), DI
	MOVBQZX relu+48(FP), R8
	SHLQ $2, DI             // row stride in bytes
	VPXORQ Z15, Z15, Z15    // zero, for relu
	XORQ R9, R9             // column offset in bytes

blk64:
	MOVQ DI, R10
	SUBQ R9, R10            // bytes of columns left
	CMPQ R10, $256
	JL   blk32
	VPXORQ Z0, Z0, Z0
	VPXORQ Z1, Z1, Z1
	VPXORQ Z2, Z2, Z2
	VPXORQ Z3, Z3, Z3
	VPXORQ Z6, Z6, Z6
	VPXORQ Z7, Z7, Z7
	VPXORQ Z8, Z8, Z8
	VPXORQ Z9, Z9, Z9
	LEAQ (AX)(R9*1), R11    // wt row i, this column block
	MOVQ BX, R12            // &x[i]
	MOVQ SI, R13
	SHRQ $1, R13            // pairs
	JZ   tail64

pair64:
	VBROADCASTSS (R12), Z4
	VBROADCASTSS 4(R12), Z5
	VFMADD231PS (R11), Z4, Z0
	VFMADD231PS 64(R11), Z4, Z1
	VFMADD231PS 128(R11), Z4, Z2
	VFMADD231PS 192(R11), Z4, Z3
	ADDQ DI, R11
	VFMADD231PS (R11), Z5, Z6
	VFMADD231PS 64(R11), Z5, Z7
	VFMADD231PS 128(R11), Z5, Z8
	VFMADD231PS 192(R11), Z5, Z9
	ADDQ DI, R11
	ADDQ $8, R12
	DECQ R13
	JNZ  pair64

tail64:
	TESTQ $1, SI
	JZ   sum64
	VBROADCASTSS (R12), Z4
	VFMADD231PS (R11), Z4, Z0
	VFMADD231PS 64(R11), Z4, Z1
	VFMADD231PS 128(R11), Z4, Z2
	VFMADD231PS 192(R11), Z4, Z3

sum64:
	VADDPS Z6, Z0, Z0       // even + odd
	VADDPS Z7, Z1, Z1
	VADDPS Z8, Z2, Z2
	VADDPS Z9, Z3, Z3
	VADDPS (CX)(R9*1), Z0, Z0   // acc + (even + odd)
	VADDPS 64(CX)(R9*1), Z1, Z1
	VADDPS 128(CX)(R9*1), Z2, Z2
	VADDPS 192(CX)(R9*1), Z3, Z3
	TESTQ R8, R8
	JZ   st64
	VMAXPS Z15, Z0, Z0
	VMAXPS Z15, Z1, Z1
	VMAXPS Z15, Z2, Z2
	VMAXPS Z15, Z3, Z3

st64:
	VMOVUPS Z0, (DX)(R9*1)
	VMOVUPS Z1, 64(DX)(R9*1)
	VMOVUPS Z2, 128(DX)(R9*1)
	VMOVUPS Z3, 192(DX)(R9*1)
	ADDQ $256, R9
	JMP  blk64

blk32:
	CMPQ R10, $128
	JL   done
	VPXORQ Z0, Z0, Z0
	VPXORQ Z1, Z1, Z1
	VPXORQ Z6, Z6, Z6
	VPXORQ Z7, Z7, Z7
	LEAQ (AX)(R9*1), R11
	MOVQ BX, R12
	MOVQ SI, R13
	SHRQ $1, R13
	JZ   tail32

pair32:
	VBROADCASTSS (R12), Z4
	VBROADCASTSS 4(R12), Z5
	VFMADD231PS (R11), Z4, Z0
	VFMADD231PS 64(R11), Z4, Z1
	ADDQ DI, R11
	VFMADD231PS (R11), Z5, Z6
	VFMADD231PS 64(R11), Z5, Z7
	ADDQ DI, R11
	ADDQ $8, R12
	DECQ R13
	JNZ  pair32

tail32:
	TESTQ $1, SI
	JZ   sum32
	VBROADCASTSS (R12), Z4
	VFMADD231PS (R11), Z4, Z0
	VFMADD231PS 64(R11), Z4, Z1

sum32:
	VADDPS Z6, Z0, Z0
	VADDPS Z7, Z1, Z1
	VADDPS (CX)(R9*1), Z0, Z0
	VADDPS 64(CX)(R9*1), Z1, Z1
	TESTQ R8, R8
	JZ   st32
	VMAXPS Z15, Z0, Z0
	VMAXPS Z15, Z1, Z1

st32:
	VMOVUPS Z0, (DX)(R9*1)
	VMOVUPS Z1, 64(DX)(R9*1)
	ADDQ $128, R9
	JMP  blk64

done:
	VZEROUPPER
	RET

// func bagAVX512(y, table *float32, feats *feat, n, rowLimit, width int)
//
// y[:width] += Σ feats[i].Value · table[row*width : (row+1)*width] over the
// features whose Row < rowLimit, in feature order, the running sum held in
// registers across the whole bag. width is 32 or 64. feat is 8 bytes: Row
// uint16 at 0, Value float32 at 4.
TEXT ·bagAVX512(SB), NOSPLIT, $0-48
	MOVQ y+0(FP), DX
	MOVQ table+8(FP), AX
	MOVQ feats+16(FP), SI
	MOVQ n+24(FP), CX
	MOVQ rowLimit+32(FP), DI
	MOVQ width+40(FP), R8
	SHLQ $2, R8             // row stride in bytes
	CMPQ R8, $256
	JE   bag64

	// width 32
	VMOVUPS (DX), Z0
	VMOVUPS 64(DX), Z1
	TESTQ CX, CX
	JZ   bst32
bl32:
	MOVWQZX (SI), R10
	CMPQ R10, DI
	JAE  bnext32
	IMULQ R8, R10
	VBROADCASTSS 4(SI), Z4
	VFMADD231PS (AX)(R10*1), Z4, Z0
	VFMADD231PS 64(AX)(R10*1), Z4, Z1
bnext32:
	ADDQ $8, SI
	DECQ CX
	JNZ  bl32
bst32:
	VMOVUPS Z0, (DX)
	VMOVUPS Z1, 64(DX)
	VZEROUPPER
	RET

bag64:
	VMOVUPS (DX), Z0
	VMOVUPS 64(DX), Z1
	VMOVUPS 128(DX), Z2
	VMOVUPS 192(DX), Z3
	TESTQ CX, CX
	JZ   bst64
bl64:
	MOVWQZX (SI), R10
	CMPQ R10, DI
	JAE  bnext64
	IMULQ R8, R10
	VBROADCASTSS 4(SI), Z4
	VFMADD231PS (AX)(R10*1), Z4, Z0
	VFMADD231PS 64(AX)(R10*1), Z4, Z1
	VFMADD231PS 128(AX)(R10*1), Z4, Z2
	VFMADD231PS 192(AX)(R10*1), Z4, Z3
bnext64:
	ADDQ $8, SI
	DECQ CX
	JNZ  bl64
bst64:
	VMOVUPS Z0, (DX)
	VMOVUPS Z1, 64(DX)
	VMOVUPS Z2, 128(DX)
	VMOVUPS Z3, 192(DX)
	VZEROUPPER
	RET

// hsum16 reduces Z4 (16 lanes) into X4 lane 0: 16→8→4, then
// (l0+l2)+(l1+l3). The scalars mirror this tree.
#define HSUM16 \
	VEXTRACTF64X4 $1, Z4, Y5 \
	VADDPS Y5, Y4, Y4 \
	VEXTRACTF128 $1, Y4, X5 \
	VADDPS X5, X4, X4 \
	VSHUFPS $0x4E, X4, X4, X5 \
	VADDPS X5, X4, X4 \
	VSHUFPS $0xB1, X4, X4, X5 \
	VADDSS X5, X4, X4

// func ln64AVX512(y, x, w, b *float32, eps float32)
//
// y = (x - mean)·rsqrt(var + eps)·w + b over 64 lanes, fp32. The mean and
// the variance reduce as ((q0+q1)+(q2+q3)) per lane, then HSUM16.
TEXT ·ln64AVX512(SB), NOSPLIT, $0-36
	MOVQ y+0(FP), DX
	MOVQ x+8(FP), AX
	MOVQ w+16(FP), BX
	MOVQ b+24(FP), CX
	VMOVUPS (AX), Z0
	VMOVUPS 64(AX), Z1
	VMOVUPS 128(AX), Z2
	VMOVUPS 192(AX), Z3
	// mean
	VADDPS Z1, Z0, Z4
	VADDPS Z3, Z2, Z5
	VADDPS Z5, Z4, Z4
	HSUM16
	MOVL $0x3c800000, R8    // 1/64
	MOVD R8, X6
	VMULSS X6, X4, X4
	VBROADCASTSS X4, Z4     // mean
	VSUBPS Z4, Z0, Z10
	VSUBPS Z4, Z1, Z11
	VSUBPS Z4, Z2, Z12
	VSUBPS Z4, Z3, Z13
	// variance
	VMULPS Z10, Z10, Z4
	VMULPS Z11, Z11, Z5
	VMULPS Z12, Z12, Z7
	VMULPS Z13, Z13, Z8
	VADDPS Z5, Z4, Z4
	VADDPS Z8, Z7, Z7
	VADDPS Z7, Z4, Z4
	HSUM16
	VMULSS X6, X4, X4       // q/64
	VADDSS eps+32(FP), X4, X4
	VSQRTSS X4, X4, X4
	MOVL $0x3f800000, R8    // 1.0
	MOVD R8, X6
	VDIVSS X4, X6, X4       // inv = 1/sqrt
	VBROADCASTSS X4, Z4
	VMULPS Z4, Z10, Z10
	VMULPS Z4, Z11, Z11
	VMULPS Z4, Z12, Z12
	VMULPS Z4, Z13, Z13
	VMULPS (BX), Z10, Z10
	VMULPS 64(BX), Z11, Z11
	VMULPS 128(BX), Z12, Z12
	VMULPS 192(BX), Z13, Z13
	VADDPS (CX), Z10, Z10
	VADDPS 64(CX), Z11, Z11
	VADDPS 128(CX), Z12, Z12
	VADDPS 192(CX), Z13, Z13
	VMOVUPS Z10, (DX)
	VMOVUPS Z11, 64(DX)
	VMOVUPS Z12, 128(DX)
	VMOVUPS Z13, 192(DX)
	VZEROUPPER
	RET

// func dot64AVX512(a, b *float32) float32
//
// One 16-lane chain over the four quarters, then HSUM16.
TEXT ·dot64AVX512(SB), NOSPLIT, $0-20
	MOVQ a+0(FP), AX
	MOVQ b+8(FP), BX
	VMOVUPS (AX), Z4
	VMULPS (BX), Z4, Z4
	VMOVUPS 64(AX), Z0
	VFMADD231PS 64(BX), Z0, Z4
	VMOVUPS 128(AX), Z1
	VFMADD231PS 128(BX), Z1, Z4
	VMOVUPS 192(AX), Z2
	VFMADD231PS 192(BX), Z2, Z4
	HSUM16
	VMOVSS X4, ret+16(FP)
	VZEROUPPER
	RET

// func addv64AVX512(y, x *float32)
TEXT ·addv64AVX512(SB), NOSPLIT, $0-16
	MOVQ y+0(FP), DX
	MOVQ x+8(FP), AX
	VMOVUPS (DX), Z0
	VMOVUPS 64(DX), Z1
	VMOVUPS 128(DX), Z2
	VMOVUPS 192(DX), Z3
	VADDPS (AX), Z0, Z0
	VADDPS 64(AX), Z1, Z1
	VADDPS 128(AX), Z2, Z2
	VADDPS 192(AX), Z3, Z3
	VMOVUPS Z0, (DX)
	VMOVUPS Z1, 64(DX)
	VMOVUPS Z2, 128(DX)
	VMOVUPS Z3, 192(DX)
	VZEROUPPER
	RET

// func poolAccAVX512(sum, mx, c *float32)
//
// sum[:32] += 0.25·c; mx[:32] = max(mx, c).
TEXT ·poolAccAVX512(SB), NOSPLIT, $0-24
	MOVQ sum+0(FP), DX
	MOVQ mx+8(FP), BX
	MOVQ c+16(FP), AX
	MOVL $0x3e800000, R8    // 0.25
	MOVD R8, X4
	VBROADCASTSS X4, Z4
	VMOVUPS (AX), Z0
	VMOVUPS 64(AX), Z1
	VMOVUPS (DX), Z2
	VMOVUPS 64(DX), Z3
	VFMADD231PS Z0, Z4, Z2
	VFMADD231PS Z1, Z4, Z3
	VMOVUPS Z2, (DX)
	VMOVUPS Z3, 64(DX)
	VMOVUPS (BX), Z2
	VMOVUPS 64(BX), Z3
	VMAXPS Z0, Z2, Z2
	VMAXPS Z1, Z3, Z3
	VMOVUPS Z2, (BX)
	VMOVUPS Z3, 64(BX)
	VZEROUPPER
	RET

// ---- width-generic kernels (any multiple of 16 lanes) ------------------------
//
// Each has a scalar twin in kern_generic.go spelling the same arithmetic in
// the same order (bagScalar, lnNScalar, dotNScalar, addvNScalar,
// poolAccNScalar); kern_test.go holds them bit-identical.

// func bagNAVX512(y, table *float32, feats *feat, n, rowLimit, width int)
//
// bagAVX512 for any width that is a multiple of 16: column blocks of 128
// lanes (eight zmm held across the whole bag), then of 64, then of 16, the
// features walked once per block. Every lane is its own FMA chain in feature
// order, as in bagScalar, so the blocking does not change a bit.
TEXT ·bagNAVX512(SB), NOSPLIT, $0-48
	MOVQ y+0(FP), DX
	MOVQ table+8(FP), AX
	MOVQ feats+16(FP), SI
	MOVQ n+24(FP), CX
	MOVQ rowLimit+32(FP), DI
	MOVQ width+40(FP), R8
	SHLQ $2, R8             // row stride in bytes
	TESTQ CX, CX
	JZ   bndone
	XORQ R9, R9             // column offset in bytes

bn128:
	MOVQ R8, R11
	SUBQ R9, R11            // bytes of columns left
	CMPQ R11, $512
	JL   bn64
	VMOVUPS (DX)(R9*1), Z0
	VMOVUPS 64(DX)(R9*1), Z1
	VMOVUPS 128(DX)(R9*1), Z2
	VMOVUPS 192(DX)(R9*1), Z3
	VMOVUPS 256(DX)(R9*1), Z4
	VMOVUPS 320(DX)(R9*1), Z5
	VMOVUPS 384(DX)(R9*1), Z6
	VMOVUPS 448(DX)(R9*1), Z7
	LEAQ (AX)(R9*1), R12
	MOVQ SI, R13
	MOVQ CX, R14
bn128l:
	MOVWQZX (R13), R10
	CMPQ R10, DI
	JAE  bn128n
	IMULQ R8, R10
	VBROADCASTSS 4(R13), Z16
	VFMADD231PS (R12)(R10*1), Z16, Z0
	VFMADD231PS 64(R12)(R10*1), Z16, Z1
	VFMADD231PS 128(R12)(R10*1), Z16, Z2
	VFMADD231PS 192(R12)(R10*1), Z16, Z3
	VFMADD231PS 256(R12)(R10*1), Z16, Z4
	VFMADD231PS 320(R12)(R10*1), Z16, Z5
	VFMADD231PS 384(R12)(R10*1), Z16, Z6
	VFMADD231PS 448(R12)(R10*1), Z16, Z7
bn128n:
	ADDQ $8, R13
	DECQ R14
	JNZ  bn128l
	VMOVUPS Z0, (DX)(R9*1)
	VMOVUPS Z1, 64(DX)(R9*1)
	VMOVUPS Z2, 128(DX)(R9*1)
	VMOVUPS Z3, 192(DX)(R9*1)
	VMOVUPS Z4, 256(DX)(R9*1)
	VMOVUPS Z5, 320(DX)(R9*1)
	VMOVUPS Z6, 384(DX)(R9*1)
	VMOVUPS Z7, 448(DX)(R9*1)
	ADDQ $512, R9
	JMP  bn128

bn64:
	CMPQ R11, $256
	JL   bn16
	VMOVUPS (DX)(R9*1), Z0
	VMOVUPS 64(DX)(R9*1), Z1
	VMOVUPS 128(DX)(R9*1), Z2
	VMOVUPS 192(DX)(R9*1), Z3
	LEAQ (AX)(R9*1), R12
	MOVQ SI, R13
	MOVQ CX, R14
bn64l:
	MOVWQZX (R13), R10
	CMPQ R10, DI
	JAE  bn64n
	IMULQ R8, R10
	VBROADCASTSS 4(R13), Z16
	VFMADD231PS (R12)(R10*1), Z16, Z0
	VFMADD231PS 64(R12)(R10*1), Z16, Z1
	VFMADD231PS 128(R12)(R10*1), Z16, Z2
	VFMADD231PS 192(R12)(R10*1), Z16, Z3
bn64n:
	ADDQ $8, R13
	DECQ R14
	JNZ  bn64l
	VMOVUPS Z0, (DX)(R9*1)
	VMOVUPS Z1, 64(DX)(R9*1)
	VMOVUPS Z2, 128(DX)(R9*1)
	VMOVUPS Z3, 192(DX)(R9*1)
	ADDQ $256, R9
	MOVQ R8, R11
	SUBQ R9, R11

bn16:
	CMPQ R11, $64
	JL   bndone
	VMOVUPS (DX)(R9*1), Z0
	LEAQ (AX)(R9*1), R12
	MOVQ SI, R13
	MOVQ CX, R14
bn16l:
	MOVWQZX (R13), R10
	CMPQ R10, DI
	JAE  bn16n
	IMULQ R8, R10
	VBROADCASTSS 4(R13), Z16
	VFMADD231PS (R12)(R10*1), Z16, Z0
bn16n:
	ADDQ $8, R13
	DECQ R14
	JNZ  bn16l
	VMOVUPS Z0, (DX)(R9*1)
	ADDQ $64, R9
	MOVQ R8, R11
	SUBQ R9, R11
	JMP  bn16

bndone:
	VZEROUPPER
	RET

// func lnNAVX512(y, x, w, b *float32, n int, eps, invN float32)
//
// y = (x - mean)·(1/sqrt(var + eps))·w + b over n lanes (a multiple of 16),
// fp32 statistics. Chunk c (16 lanes) of the sum and of the squared
// deviations accumulates in Z(c%4); the four combine as ((Z0+Z1)+(Z2+Z3)),
// then HSUM16 -- ln64AVX512's tree when n is 64. y may be x.
TEXT ·lnNAVX512(SB), NOSPLIT, $0-48
	MOVQ y+0(FP), DX
	MOVQ x+8(FP), AX
	MOVQ w+16(FP), BX
	MOVQ b+24(FP), CX
	MOVQ n+32(FP), SI
	SHLQ $2, SI             // bytes

	// mean
	VPXORQ Z0, Z0, Z0
	VPXORQ Z1, Z1, Z1
	VPXORQ Z2, Z2, Z2
	VPXORQ Z3, Z3, Z3
	XORQ R9, R9
lnm4:
	MOVQ SI, R10
	SUBQ R9, R10
	CMPQ R10, $256
	JL   lnm1
	VADDPS (AX)(R9*1), Z0, Z0
	VADDPS 64(AX)(R9*1), Z1, Z1
	VADDPS 128(AX)(R9*1), Z2, Z2
	VADDPS 192(AX)(R9*1), Z3, Z3
	ADDQ $256, R9
	JMP  lnm4
lnm1:
	CMPQ R10, $64
	JL   lnmd
	VADDPS (AX)(R9*1), Z0, Z0
	CMPQ R10, $128
	JL   lnmd
	VADDPS 64(AX)(R9*1), Z1, Z1
	CMPQ R10, $192
	JL   lnmd
	VADDPS 128(AX)(R9*1), Z2, Z2
lnmd:
	VADDPS Z1, Z0, Z4
	VADDPS Z3, Z2, Z7
	VADDPS Z7, Z4, Z4
	HSUM16
	VMULSS invN+44(FP), X4, X4
	VBROADCASTSS X4, Z6     // mean

	// variance
	VPXORQ Z0, Z0, Z0
	VPXORQ Z1, Z1, Z1
	VPXORQ Z2, Z2, Z2
	VPXORQ Z3, Z3, Z3
	XORQ R9, R9
lnv4:
	MOVQ SI, R10
	SUBQ R9, R10
	CMPQ R10, $256
	JL   lnv1
	VMOVUPS (AX)(R9*1), Z8
	VMOVUPS 64(AX)(R9*1), Z9
	VMOVUPS 128(AX)(R9*1), Z10
	VMOVUPS 192(AX)(R9*1), Z11
	VSUBPS Z6, Z8, Z8
	VSUBPS Z6, Z9, Z9
	VSUBPS Z6, Z10, Z10
	VSUBPS Z6, Z11, Z11
	VFMADD231PS Z8, Z8, Z0
	VFMADD231PS Z9, Z9, Z1
	VFMADD231PS Z10, Z10, Z2
	VFMADD231PS Z11, Z11, Z3
	ADDQ $256, R9
	JMP  lnv4
lnv1:
	CMPQ R10, $64
	JL   lnvd
	VMOVUPS (AX)(R9*1), Z8
	VSUBPS Z6, Z8, Z8
	VFMADD231PS Z8, Z8, Z0
	CMPQ R10, $128
	JL   lnvd
	VMOVUPS 64(AX)(R9*1), Z9
	VSUBPS Z6, Z9, Z9
	VFMADD231PS Z9, Z9, Z1
	CMPQ R10, $192
	JL   lnvd
	VMOVUPS 128(AX)(R9*1), Z10
	VSUBPS Z6, Z10, Z10
	VFMADD231PS Z10, Z10, Z2
lnvd:
	VADDPS Z1, Z0, Z4
	VADDPS Z3, Z2, Z7
	VADDPS Z7, Z4, Z4
	HSUM16
	VMULSS invN+44(FP), X4, X4
	VADDSS eps+40(FP), X4, X4
	VSQRTSS X4, X4, X4
	MOVL $0x3f800000, R8    // 1.0
	MOVD R8, X7
	VDIVSS X4, X7, X4       // inv = 1/sqrt
	VBROADCASTSS X4, Z7

	// y = (x - mean)·inv·w + b
	XORQ R9, R9
lno:
	VMOVUPS (AX)(R9*1), Z8
	VSUBPS Z6, Z8, Z8
	VMULPS Z7, Z8, Z8
	VMULPS (BX)(R9*1), Z8, Z8
	VADDPS (CX)(R9*1), Z8, Z8
	VMOVUPS Z8, (DX)(R9*1)
	ADDQ $64, R9
	CMPQ R9, SI
	JL   lno
	VZEROUPPER
	RET

// func dotNAVX512(a, b *float32, n int) float32
//
// dot64AVX512 over n lanes (a multiple of 16): one 16-lane chain, the
// first chunk a product, every later one an FMA, then HSUM16.
TEXT ·dotNAVX512(SB), NOSPLIT, $0-28
	MOVQ a+0(FP), AX
	MOVQ b+8(FP), BX
	MOVQ n+16(FP), SI
	SHLQ $2, SI
	VMOVUPS (AX), Z4
	VMULPS (BX), Z4, Z4
	MOVQ $64, R9
dotl:
	CMPQ R9, SI
	JGE  dotd
	VMOVUPS (AX)(R9*1), Z0
	VFMADD231PS (BX)(R9*1), Z0, Z4
	ADDQ $64, R9
	JMP  dotl
dotd:
	HSUM16
	VMOVSS X4, ret+24(FP)
	VZEROUPPER
	RET

// func addvNAVX512(y, x *float32, n int)
//
// y[:n] += x[:n], n a multiple of 16.
TEXT ·addvNAVX512(SB), NOSPLIT, $0-24
	MOVQ y+0(FP), DX
	MOVQ x+8(FP), AX
	MOVQ n+16(FP), SI
	SHLQ $2, SI
	XORQ R9, R9
addv4:
	MOVQ SI, R10
	SUBQ R9, R10
	CMPQ R10, $256
	JL   addv1
	VMOVUPS (DX)(R9*1), Z0
	VMOVUPS 64(DX)(R9*1), Z1
	VMOVUPS 128(DX)(R9*1), Z2
	VMOVUPS 192(DX)(R9*1), Z3
	VADDPS (AX)(R9*1), Z0, Z0
	VADDPS 64(AX)(R9*1), Z1, Z1
	VADDPS 128(AX)(R9*1), Z2, Z2
	VADDPS 192(AX)(R9*1), Z3, Z3
	VMOVUPS Z0, (DX)(R9*1)
	VMOVUPS Z1, 64(DX)(R9*1)
	VMOVUPS Z2, 128(DX)(R9*1)
	VMOVUPS Z3, 192(DX)(R9*1)
	ADDQ $256, R9
	JMP  addv4
addv1:
	CMPQ R9, SI
	JGE  addvd
	VMOVUPS (DX)(R9*1), Z0
	VADDPS (AX)(R9*1), Z0, Z0
	VMOVUPS Z0, (DX)(R9*1)
	ADDQ $64, R9
	JMP  addv1
addvd:
	VZEROUPPER
	RET

// func poolAccNAVX512(sum, mx, c *float32, n int)
//
// poolAccAVX512 over n lanes (a multiple of 16): sum += 0.25·c, mx = max(mx, c).
TEXT ·poolAccNAVX512(SB), NOSPLIT, $0-32
	MOVQ sum+0(FP), DX
	MOVQ mx+8(FP), BX
	MOVQ c+16(FP), AX
	MOVQ n+24(FP), SI
	SHLQ $2, SI
	MOVL $0x3e800000, R8    // 0.25
	MOVD R8, X4
	VBROADCASTSS X4, Z4
	XORQ R9, R9
pnl:
	VMOVUPS (AX)(R9*1), Z0
	VMOVUPS (DX)(R9*1), Z2
	VFMADD231PS Z0, Z4, Z2
	VMOVUPS Z2, (DX)(R9*1)
	VMOVUPS (BX)(R9*1), Z3
	VMAXPS Z0, Z3, Z3
	VMOVUPS Z3, (BX)(R9*1)
	ADDQ $64, R9
	CMPQ R9, SI
	JL   pnl
	VZEROUPPER
	RET
