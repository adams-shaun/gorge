package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A printed trigger's own "{N} and ExileFromGrave<1/Spec>" cost (Sylvia Tilly,
// Joy of Discovery). gorge's triggered-cost window (rules/cumulative.go) reads
// a cost with an exile part as a component cost: it is offered only when the
// floating pool alone covers the mana half and the graveyard holds the card
// (no mana window opens for it), the payer is asked pay or decline, the
// "exile_cost" pick follows (unasked with exactly one candidate), and the mana
// half is charged last (settleTriggeredMandatory).
//
// The program is the PayMana instruction (the election) followed by a
// PayExileGrave instruction carrying kop.PayBSubChain (the pick, which also
// charges the mana on the way out).

// exileChain returns the chained exile instruction after the current PayMana
// instruction, or nil when the current instruction is a plain mana cost.
//
//go:noinline
func (s *State) exileChain(k *prep) *kop.Instr {
	pr := s.prog(k)
	i := int(s.res.pc) + 1
	if i >= len(pr) {
		return nil
	}
	if n := &pr[i]; n.Op == kop.OpIfPaid && n.C&0xF == kop.PayExileGrave && n.B.N == kop.PayBSubChain {
		return n
	}
	return nil
}

// execPayChain poses the pay or decline election of a mana-and-exile cost:
// "pay" is offered only while the pool pays the mana half and the graveyard
// holds the card.
//
//go:noinline
func (s *State) execPayChain(k *prep, p uint8, ch *kop.Instr, skip uint16, amt int, pips int16) bool {
	c := manaOf(amt, pips)
	if !s.poolPays(p, &c) || s.countGraveExile(k, p, ch.F) < s.value(k, ch.A) {
		skip |= pfNoPay // still asked, decline only (gorge poses it)
	}
	s.g.pickF = skip
	s.g.pickZ = kop.PayMana
	s.g.pickLeft = uint8(amt)
	s.g.pickP = p
	s.res.ask, s.res.askP = raGenPay, p
	return false
}

// startChainExile answers "pay": the card pick (gorge's exile_cost), unasked
// with exactly the needed candidates. The mana is charged when it is done
// (chainCharge).
//
//go:noinline
func (s *State) startChainExile(k *prep, ch *kop.Instr) {
	s.res.pc++ // the pick reads the chained instruction's filter
	s.g.pickZ = kop.PayExileGrave
	s.g.pickLeft = uint8(s.value(k, ch.A))
	s.res.ask = raGenPayPick
	if s.autoPayPick(k) {
		s.payDone(k, true)
	}
}

// chainCharge charges the mana half of a mana-and-exile cost once its card is
// exiled: the current instruction is the chained one, the mana is the one
// before it.
//
//go:noinline
func (s *State) chainCharge(k *prep) {
	pr := s.prog(k)
	pc := int(s.res.pc)
	if pc == 0 || pr[pc].B.N != kop.PayBSubChain {
		return
	}
	m := &pr[pc-1]
	c := manaOf(s.value(k, m.A), m.B.N)
	s.payFromPool(s.g.pickP, &c, 0)
}
