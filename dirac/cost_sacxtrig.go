package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Sac<X/Spec> as a trigger's own whole cost (Sephiroth, One-Winged Angel: "you
// may sacrifice any number of other creatures. If you do, draw that many
// cards"). Gorge's triggered-cost window (rules cumulative.go
// triggeredCostXAsk, costCarriesSacX) announces X BEFORE the pay / don't-pay
// election, a "trigger_cost_x" KChoose over 0..min(64, the permanents the
// sacrifice may name); the answer folds into a literal Sac<X/Spec>, X = 0
// leaves the election decline-only (the component gate refuses a zero count),
// and a paid election sacrifices X picks through the ordinary PaySac stage.
// The body reads Count$xPaid as the announced X. The cost is OpIfPaid with
// PaySac and B.N PayBSubSacXTrig, F the sacrifice filter; the announcement is
// trig_maypayx.go's raTrigX. Cold: reached only from that PaySac instruction.

// payKindSacX is the pay kind (execPay's kind, pickZ) while the announcement
// of a Sac<X> trigger cost is open; the answer turns it into kop.PaySac.
const payKindSacX = 28

// execTrigSacX poses the X announcement: 0..the permanents filter f names
// (capped). False = suspended.
//
//go:noinline
func (s *State) execTrigSacX(k *prep, p uint8, skip uint16, f kop.FilterID) bool {
	s.g.pickF = skip
	s.g.pickZ = payKindSacX
	s.g.pickLeft = uint8(min(trigXCap, s.countSac(k, p, f, s.resSrcSlot(k))))
	s.g.pickP = p
	s.res.ask, s.res.askP = raTrigX, p
	return false
}

// applyAskSacX folds the announced X into a literal sacrifice count, binds it
// as the resolving trigger's Count$xPaid (the body runs only if the cost is
// paid, and the pick stage counts pickLeft down), and poses the pay / don't-pay
// election: a zero count is decline-only.
//
//go:noinline
func (s *State) applyAskSacX(k *prep, a Action) bool {
	x := a.Mode()
	p := s.g.pickP
	s.g.pickLeft = x
	s.g.pickZ = kop.PaySac
	if x == 0 || s.countSac(k, p, s.prog(k)[s.res.pc].F, s.resSrcSlot(k)) < int(x) {
		s.g.pickF |= pfNoPay
	}
	s.res.e.x = objWord(x)
	s.res.ask, s.res.askP = raGenPay, p
	return true
}
