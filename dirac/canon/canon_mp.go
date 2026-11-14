//go:build dirac_mp

package canon

// Ref is a canonical reference (canon.go): nine bits in a uint16 under
// dirac_mp, as dirac.TCode.
type Ref = uint16

// Canon is a canonical option tuple (canon.go): kind 8 | source 9 |
// target 9 | mode 8 bits, kind<<26 | source<<17 | target<<8 | mode, which
// needs 34 bits.
type Canon uint64

const (
	canonKindShift = 26
	canonSrcShift  = 17
	canonTgtShift  = 8
	canonRefMask   = 0x1ff
)
