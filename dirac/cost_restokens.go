package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Resolution cost tokens that are payments the kernel already performs as
// effects: a ward's AddCounterYou<N/POISON> (the payer's poison counters), an
// UnlessCost$ DamageYou<X> (the resolving source's damage to the payer) and
// Draw<X/You> (kop.PayDraw with a computed count). Each is one pay / decline
// election; the payments are atomic and never ask a second question.

// wardPoisBit marks a ward trigger amount (stackEntry.trigAmt) as poison N
// (the low byte): the payer gets N poison counters. Bit 8 is clear in every
// other ward form except wardMLBit's packed life (checked first by
// resolveWard), a mana ward is N >= 0 and a life ward -N.
const wardPoisBit int16 = 0x0100

// payKindPoison is the pay kind (execPay's kind, pickZ) of a ward's poison
// payment; payKindDamage that of an unless cost's damage to the payer.
const (
	payKindPoison = 41
	payKindDamage = 40
)

// resTokenPayKind maps a PayDraw whose B.N carries kop.PayBSubDamage to
// payKindDamage (an UnlessCost$ DamageYou<X>).
func resTokenPayKind(kind int, bn int16) int {
	if kind == kop.PayDraw && bn == kop.PayBSubDamage {
		return payKindDamage
	}
	return kind
}

// resolveWardPoison poses the ward's pay / don't-pay election. Poison
// counters are always payable (gorge's ward offers both and pays at once).
//
//go:noinline
func (s *State) resolveWardPoison(k *prep) {
	e := &s.res.e
	if s.wardedIndex(e) < 0 {
		s.finishResolve(k, false)
		return
	}
	s.g.pickF = pfWard
	s.g.pickZ = payKindPoison
	s.g.pickLeft = uint8(e.trigAmt &^ wardPoisBit)
	s.g.pickP = e.trigPl
	s.res.ask, s.res.askP = raGenPay, e.trigPl
	s.ctl = ctlResolve
}

// payResToken pays the accepted election: the ward's poison counters to the
// payer (through addPoison, so a "can't get poison counters" replacement
// applies) or the resolving source's damage to the payer (damageTo: the
// prevention and replacement pass, infect, lifelink and the damage triggers).
//
//go:noinline
func (s *State) payResToken(k *prep) {
	if s.g.pickZ == payKindPoison {
		s.addPoison(s.g.pickP, int(s.g.pickLeft))
		return
	}
	s.damageTo(k, PlayerTarget0+s.g.pickP, int(s.g.pickLeft))
}
