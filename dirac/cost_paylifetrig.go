package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// PayLife<X> as a trigger's own whole cost (Necrodominance: "you may pay any
// amount of life. If you do, draw that many cards"). Gorge's triggered-cost
// window (rules cumulative.go triggeredCostXAsk, the SVar:X = Count$xPaid
// payer-chooses arm) announces X BEFORE the pay / don't-pay election, a
// "trigger_cost_x" KChoose over 0..min(64, the payer's life); the answer folds
// into a literal life cost, the election follows (pay is offered at X = 0
// too, a free cost) and a paid election loses X life. The body reads
// Count$xPaid as the announced X. The cost is OpIfPaid with PayManaX and B.N
// PayBSubLifeX, A the cap; the announcement is trig_maypayx.go's raTrigX.
// Cold: reached only from that PayManaX instruction.

// payKindLifeX is the pay kind (execPay's kind, pickZ) while the announcement
// of a PayLife<X> trigger cost is open; the answer turns it into kop.PayLife.
const payKindLifeX = 27

// execTrigLifeX poses the X announcement: 0..the payer's life, capped. False =
// suspended.
//
//go:noinline
func (s *State) execTrigLifeX(k *prep, p uint8, skip uint16, cap int) bool {
	s.g.pickF = skip
	s.g.pickZ = payKindLifeX
	s.g.pickLeft = uint8(max(0, min(cap, trigXCap, int(s.pl[SeatIx(p)].life))))
	s.g.pickP = p
	s.res.ask, s.res.askP = raTrigX, p
	return false
}

// applyAskLifeX folds the announced X into a literal life cost and poses the
// pay / don't-pay election on it.
//
//go:noinline
func (s *State) applyAskLifeX(k *prep, a Action) bool {
	s.g.pickLeft = a.Mode()
	s.g.pickZ = kop.PayLife
	s.res.ask, s.res.askP = raGenPay, s.g.pickP
	return true
}

// lifeXBind makes the announced X the resolving trigger's Count$xPaid (the
// paid life stage kept it in pickLeft).
//
//go:noinline
func (s *State) lifeXBind(k *prep) {
	if in := &s.prog(k)[s.res.pc]; in.C&0xF == kop.PayManaX && in.B.N == kop.PayBSubLifeX {
		s.res.e.x = objWord(s.g.pickLeft)
	}
}
