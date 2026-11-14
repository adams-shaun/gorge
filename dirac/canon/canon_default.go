//go:build !dirac_mp

package canon

// Ref is a canonical reference (canon.go): a byte in the default class, as
// dirac.TCode.
type Ref = uint8

// Canon is a canonical option tuple (canon.go): kind<<24 | source<<16 |
// target<<8 | mode.
type Canon uint32

const (
	canonKindShift = 24
	canonSrcShift  = 16
	canonTgtShift  = 8
	canonRefMask   = 0xff
)
