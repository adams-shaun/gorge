package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// UnlessCost$ Mill<N> as a resolution-time cost (CR 701.17b, 118.3; Deep
// Spawn's "sacrifice CARDNAME unless you mill two cards"): gorge poses the pay
// / decline election (pay only while pay.MillCostPayable holds), then the
// no-choice path payUnlessCost settles it through the shared pay.PayMillCost —
// the payer mills N cards from the top of their own library, one real move per
// card in top-first order, sharing a mill batch. It rides PayDraw with B.N
// kop.PayBSubMill (C's nibble is full); millPayKind widens it to payKindMill.

// payKindMill is the pay kind (execPay's kind, pickZ) of an UnlessCost$ Mill.
const payKindMill = 24

// millPayKind widens a PayDraw whose B.N is PayBSubMill to payKindMill.
func millPayKind(kind int, bn int16) int {
	if kind == kop.PayDraw && bn == kop.PayBSubMill {
		return payKindMill
	}
	return kind
}

// unlessMillPayable is the offer gate (gorge unlessComponentsPayable's
// pay.MillCostPayable, CR 701.17b): the payer's library holds at least the
// count, so a short library leaves the election decline-only.
func (s *State) unlessMillPayable(p uint8, amt int) bool {
	return int(s.pl[p].libLen()) >= amt
}

// payUnlessMill settles an accepted UnlessCost$ Mill<N> election: the payer
// mills the count from the top of their own library (gorge payUnlessCost's
// shared pay.PayMillCost after the election, CR 701.17b). The offer gate
// priced the library, so the count below is the same guard PayMillCost keeps.
//
//go:noinline
func (s *State) payUnlessMill(k *prep) {
	s.millCostPay(k, s.g.pickP, int(s.g.pickLeft))
}

// payDrawOrMill settles a PayDraw payment, or the UnlessCost$ Mill that rides
// the same instruction widened to payKindMill (millPayKind): the payer either
// draws the marked count or mills it from their own library's top.
func (s *State) payDrawOrMill(k *prep) {
	if s.g.pickZ == payKindMill {
		s.payUnlessMill(k)
		return
	}
	s.payDrawToken(k)
}
