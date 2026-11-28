package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// kfix-cast: "any player may pay" (UnlessPayer$ Player; kgen lowers it to
// OpIfPaid / OpUnlessPaid / OpUnlessSwitched obj=RefEachPlayer). gorge at the pin
// (engine/effects/unless.go unlessPayerTargetsFor, unlessProceed): the payers
// are every living player in AliveFrom(0) seat order -- player 0 first,
// whoever controls the resolving object, a departed player skipped (CR
// 800.4a, 800.4f) -- and each is posed the unless ask (KModes, ResumeKind
// "unless_pay", decline only when the cost can't be paid); a pay ends the
// walk paid, a decline asks the next payer, and only when every payer
// declined does the body take the unpaid branch.
// The ask is the ordinary raGenPay with pfModes (gorge's unless ask, never
// a trigger's own cost) and pfEachNext while a later seat is still to come;
// g.pickP is the seat being asked, so the walk needs no state of its own.
// Guard: prep.eachPay (some op has the shape); the entry is execPay's
// cold p == 255 branch and payDone's decline.

// execPayEach poses the any-player ask to the first living seat; false =
// suspended.
//
//go:noinline
func (s *State) execPayEach(k *prep, in *kop.Instr, kind int, skip uint16, amt int) bool {
	s.posePayEach(k, in, s.eachPayerFrom(0), kind, skip|pfModes, amt)
	return false
}

// payEachNext: the asked seat declined; the next living seat is asked the
// same cost.
//
//go:noinline
func (s *State) payEachNext(k *prep) {
	in := &s.prog(k)[s.res.pc]
	f := s.g.pickF &^ (pfEachNext | pfNoPay)
	s.posePayEach(k, in, s.eachPayerFrom(s.g.pickP+1), int(s.g.pickZ), f, s.value(k, in.A))
}

// eachPayerFrom is the first seat at or after p, in seat order and without
// wrapping, that is still in the game, or noSeat past the last.
func (s *State) eachPayerFrom(p uint8) uint8 {
	for ; p < s.nSeats(); p++ {
		if !s.out(p) {
			return p
		}
	}
	return noSeat
}

// posePayEach is posePay for seat p of the walk, marking whether a later seat
// is still to be asked.
func (s *State) posePayEach(k *prep, in *kop.Instr, p uint8, kind int, skip uint16, amt int) {
	if s.eachPayerFrom(p+1) != noSeat {
		skip |= pfEachNext
	}
	s.posePay(k, in, p, kind, skip, amt)
}

// posePay sets the raGenPay ask for payer p (execPay's tail).
func (s *State) posePay(k *prep, in *kop.Instr, p uint8, kind int, skip uint16, amt int) {
	stall := uint8(0)
	if !s.canPayGen(k, p, kind, amt, in.B.N, in.F) {
		var ok bool
		ok, stall = s.windowPays(k, p, kind, skip, amt, in.B.N)
		if !ok {
			skip |= pfNoPay
		}
	}
	if stall != 0 {
		// the drop is final (the payer is offered decline only) and a failed
		// window probe's fork latched a rule gap the kernel stalls by name:
		// declare it, not a silent unpayable cost. A found plan's spill is the
		// same model gap and is declared too.
		s.capFull(stall)
	}
	s.g.pickF = skip
	s.g.pickZ = uint8(kind)
	s.g.pickLeft = uint8(amt)
	s.g.pickP = p
	s.res.ask, s.res.askP = raGenPay, p
}

// prepEachPay: some ability of t carries an any-player pay op.
func prepEachPay(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if (in.Op == kop.OpIfPaid || in.Op == kop.OpUnlessPaid || in.Op == kop.OpUnlessSwitched) && in.Obj == kop.RefEachPlayer {
					return true
				}
			}
		}
	}
	return false
}
