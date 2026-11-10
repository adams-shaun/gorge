package kop

// An UnlessCost$ AddCounter<N/KIND/recipient> (CR 118.12a, 122.1) puts A
// counters of a kind on the resolving source or on the parent ability's
// target. PayTapX's C nibble is full at 15, so the instruction rides PayTapX
// and its B.N packs the whole encoding (the instruction layout is unchanged):
//
//	bits 0-3  PayBSubAddCounter
//	bits 4-5  recipient: 0 the source, 1..3 target slot 0..2
//	bits 6-13 the CounterKind
//
// F stays 0: it is a filter operand and the filter walkers read it as one.

// PayAddCounter is the pay kind of an unless AddCounter; the kernel reaches it
// through PayTapX and PayBSubAddCounter (dirac/cost_unlessaddcounter.go).
const PayAddCounter = 16

// PayBSubAddCounter marks a PayTapX unless-cost whose B.N is an AddCounter
// encoding (PayAddCounterB).
const PayBSubAddCounter = 15

// PayAddCounterB packs the counter kind and recipient into a B.N. The
// recipient is RefSelf or one of RefTarget0..RefTarget2; ok is false for any
// other Ref.
func PayAddCounterB(kind CounterKind, recipient Ref) (n int, ok bool) {
	r := 0
	switch recipient {
	case RefSelf:
	case RefTarget0, RefTarget1, RefTarget2:
		r = int(recipient-RefTarget0) + 1
	default:
		return 0, false
	}
	return PayBSubAddCounter | r<<4 | int(kind)<<6, true
}

// IsPayAddCounterB reports whether a PayTapX's B.N is an AddCounter encoding.
func IsPayAddCounterB(n int16) bool { return n&0xf == PayBSubAddCounter }

// PayAddCounterFrom unpacks a PayAddCounterB encoding.
func PayAddCounterFrom(n int16) (kind CounterKind, recipient Ref) {
	recipient = RefSelf
	if r := Ref(n>>4) & 3; r != 0 {
		recipient = RefTarget0 + r - 1
	}
	return CounterKind(uint16(n) >> 6 & 0xff), recipient
}
