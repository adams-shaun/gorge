package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// kfix-cast: "any player may pay" (UnlessPayer$ Player; kgen lowers it to
// OpIfPaid / OpUnlessPaid / OpUnlessSwitched obj=RefEachPlayer). gorge at the pin
// (engine/effects/unless.go unlessPayerTargets, unlessProceed): the payers
// are every living player in AliveFrom(0) seat order -- player 0 first,
// whoever controls the resolving object -- and each is posed the unless
// ask (KModes, ResumeKind "unless_pay", decline only when the cost can't
// be paid); a pay ends the walk paid, a decline asks the next payer, and
// only when every payer declined does the body take the unpaid branch.
// The ask is the ordinary raGenPay with pfModes (gorge's unless ask, never
// a trigger's own cost) and pfEachNext while player 1 is still to come.
// Guard: prep.eachPay (some op has the shape); the entry is execPay's
// cold p == 255 branch and payDone's decline.

// execPayEach poses the any-player ask to player 0; false = suspended.
//
//go:noinline
func (s *State) execPayEach(k *prep, in *kop.Instr, kind int, skip uint16, amt int) bool {
	s.posePay(k, in, 0, kind, skip|pfModes|pfEachNext, amt)
	return false
}

// payEachNext: player 0 declined; player 1 is asked the same cost.
//
//go:noinline
func (s *State) payEachNext(k *prep) {
	in := &s.prog(k)[s.res.pc]
	f := s.g.pickF &^ (pfEachNext | pfNoPay)
	s.posePay(k, in, 1, int(s.g.pickZ), f, s.value(k, in.A))
}

// posePay sets the raGenPay ask for payer p (execPay's tail).
func (s *State) posePay(k *prep, in *kop.Instr, p uint8, kind int, skip uint16, amt int) {
	if !s.canPayGen(k, p, kind, amt, in.B.N, in.F) && !s.windowPays(k, p, kind, skip, amt, in.B.N) {
		skip |= pfNoPay
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
