package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// kfix-cast: "any player may pay" (UnlessPayer$ Player, and UnlessPayer$
// Opponent; kgen lowers them to OpIfPaid / OpUnlessPaid / OpUnlessSwitched
// obj=RefEachPlayer or RefEachOpponent). gorge at the pin
// (engine/effects/unless.go unlessPayerTargetsFor, unlessProceed): the payers
// are every living player (for Opponent, without the resolving controller) in
// APNAP order (CR 101.4): the active player first, then turn order, a departed
// player skipped (CR 800.4a, 800.4f). Each is posed the unless ask (KModes,
// ResumeKind "unless_pay", decline only when the cost can't be paid); a pay
// ends the walk paid, a decline asks the next payer, and only when every payer
// declined does the body take the unpaid branch.
// The ask is the ordinary raGenPay with pfModes (gorge's unless ask, never
// a trigger's own cost) and pfEachNext while a later seat is still to come;
// g.pickP is the seat being asked, so the walk needs no state of its own.
// A walk of one payer (every two-seat "opponent" ask) is execPay's ordinary
// single-payer ask.
// Guard: prep.eachPay (some op has the shape); the entry is execPay's
// cold p == 255 branch and payDone's decline.

// eachPayers is in's payers in the order they are asked: the living seats in
// turn order from the active player, the resolving controller left out of an
// opponents' walk.
func (s *State) eachPayers(in *kop.Instr) seatSeq {
	all := s.aliveFrom(s.active)
	if in.Obj != kop.RefEachOpponent {
		return all
	}
	var q seatSeq
	for i := uint8(0); i < all.n; i++ {
		if p := all.at(i); p != s.res.e.ctrl {
			q.add(p)
		}
	}
	return q
}

// eachPayAfter is the payer asked after p, or noSeat past the last.
func (s *State) eachPayAfter(in *kop.Instr, p uint8) uint8 {
	q := s.eachPayers(in)
	for i := uint8(0); i < q.n; i++ {
		if q.at(i) == p {
			return q.at(i + 1)
		}
	}
	return noSeat
}

// execPayStart is execPay's payer: the one seat in asks, or 255 with walk set
// when in asks several in turn, or 255 alone when nobody is asked (a departed
// payer, cost_departedpayer.go; an each-payer ref with no living payer).
func (s *State) execPayStart(k *prep, in *kop.Instr) (p uint8, walk bool) {
	if !k.eachPay || in.Obj != kop.RefEachPlayer && in.Obj != kop.RefEachOpponent {
		return s.execPayer(k, in.Obj), false
	}
	q := s.eachPayers(in)
	if q.n > 1 {
		return 255, true
	}
	return q.at(0), false
}

// execPayEach poses the walk's ask to its first payer; false = suspended.
//
//go:noinline
func (s *State) execPayEach(k *prep, in *kop.Instr, kind int, skip uint16, amt int) bool {
	s.posePayEach(k, in, s.eachPayers(in).at(0), kind, skip|pfModes, amt)
	return false
}

// payEachNext: the asked seat declined; the next payer is asked the same
// cost.
//
//go:noinline
func (s *State) payEachNext(k *prep) {
	in := &s.prog(k)[s.res.pc]
	f := s.g.pickF &^ (pfEachNext | pfNoPay)
	kind := int(s.g.pickZ)
	if kind == payPickBlight {
		kind = kop.PaySac // poseGenPay's remap, undone for the next payer's gate
	}
	s.posePayEach(k, in, s.eachPayAfter(in, s.g.pickP), kind, f, max(s.value(k, in.A), 0))
}

// posePayEach is posePay for seat p of the walk, marking whether a later seat
// is still to be asked.
func (s *State) posePayEach(k *prep, in *kop.Instr, p uint8, kind int, skip uint16, amt int) {
	if s.eachPayAfter(in, p) != noSeat {
		skip |= pfEachNext
	}
	s.posePay(k, in, p, kind, skip, amt)
}

// posePay asks payer p for in's cost: the gate and the ask execPay's
// single-payer tail makes (payAnswerGate, poseGenPay).
func (s *State) posePay(k *prep, in *kop.Instr, p uint8, kind int, skip uint16, amt int) {
	skip = s.payAnswerGate(k, in, p, kind, skip, amt, in.B.N)
	s.poseGenPay(in, p, kind, skip, amt)
}

// poseGenPay sets the raGenPay ask for payer p, skip already gated
// (payAnswerGate).
func (s *State) poseGenPay(in *kop.Instr, p uint8, kind int, skip uint16, amt int) {
	s.g.pickF = skip
	s.g.pickZ = uint8(kind)
	if kind == kop.PaySac && in.B.N == kop.PayBSubBlight {
		s.g.pickZ = payPickBlight
		s.g.pickLeft = 1 // one creature, however many counters
	} else {
		s.g.pickLeft = uint8(amt)
	}
	s.g.pickP = p
	s.res.ask, s.res.askP = raGenPay, p
}

// prepEachPay: some ability of t carries an any-player (or any-opponent) pay op.
func prepEachPay(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if (in.Op == kop.OpIfPaid || in.Op == kop.OpUnlessPaid || in.Op == kop.OpUnlessSwitched) && (in.Obj == kop.RefEachPlayer || in.Obj == kop.RefEachOpponent) {
					return true
				}
			}
		}
	}
	return false
}
