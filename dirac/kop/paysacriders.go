package kop

// An UnlessCost$ whose Sac<N/Spec> part rides beside a PayLife<L> and/or a
// Draw<D/Who> part (Tresserhorn's Lord, Returned: "sacrifice him unless you
// sacrifice three creatures, pay 3 life, and have target opponent draw three
// cards"). gorge poses ONE pay / decline election for the whole cost, then the
// Sac part's pick, then charges life, sacrifices and draws in that order
// (pay/unlessflow.go AdvanceUnless), so the kernel carries the riders on the
// Sac instruction itself instead of chaining elections. The instruction is a
// PaySac whose B.N packs the rider amounts (the instruction layout is
// unchanged; the draw's recipient rides W, the payer when W is RefNone):
//
//	bits 0-4   PayBSubSacRiders
//	bits 5-9   L, the life paid (0 = no life part)
//	bits 10-14 D, the cards drawn (0 = no draw part)
//
// dirac/cost_unlesssacriders.go reads it; kgen's unlessPartChain writes it.

// PayBSubSacRiders marks a PaySac unless-cost whose B.N is a PaySacRidersB
// encoding.
const PayBSubSacRiders = 18

// PaySacRidersB packs the rider amounts into a B.N; ok is false for an amount
// outside 0..31 (five bits each) or when both are absent (a lone Sac needs no
// riders).
func PaySacRidersB(life, draw int) (n int, ok bool) {
	if life < 0 || draw < 0 || life > 31 || draw > 31 || life == 0 && draw == 0 {
		return 0, false
	}
	return PayBSubSacRiders | life<<5 | draw<<10, true
}

// IsPaySacRidersB reports whether a PaySac's B.N is a PaySacRidersB encoding.
func IsPaySacRidersB(n int16) bool { return n >= 0 && n&0x1f == PayBSubSacRiders }

// PaySacRidersFrom unpacks a PaySacRidersB encoding.
func PaySacRidersFrom(n int16) (life, draw int) {
	return int(n>>5) & 0x1f, int(n>>10) & 0x1f
}
