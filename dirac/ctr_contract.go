package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Contract counters (Miss Highwater): a player gets one from an OpPutCounter
// of OpExt ExtPutContract on a player operand, and a combat DamageDone line with
// kop.Tf5DmgNoContract (ValidTarget$ Player.counters_EQ0_Contract) fires
// only on a damaged player who has none. gorge keeps the count in
// state.Player.Counters ("Contract", one PlayerCounterChange per put) and
// its proliferate offers a player holding one (effects hasRealCounters),
// so the kernel's proliferate does too.
//
// The record is a cold array of one count per seat, kept only when
// genPrep.anyContract.

// contractCold is the per-seat count, or nil when the table keeps none.
func (s *State) contractCold() []uint8 { return coldContract.of(s) }

var coldContract = newCold[uint8]("contract", func(g *genPrep) int {
	if g.anyContract {
		return MaxSeats
	}
	return 0
})

// contracts is seat p's contract counter count.
func (s *State) contracts(p uint8) uint8 {
	if c := s.contractCold(); c != nil && p < MaxSeats {
		return c[SeatIx(p)]
	}
	return 0
}

// addContract gives seat p n contract counters. A counter doubler on its
// controller's own counters (rbDblCtrYou) applies as it does to poison
// (addPoison).
func (s *State) addContract(k *prep, p uint8, n int) {
	c := s.contractCold()
	if c == nil || n <= 0 || p >= MaxSeats {
		return
	}
	if k.gen != nil && k.gen.anyRepl&rbDblCtrYou != 0 {
		for d := s.dblCtrYou(k, s.counterAdder()); d > 0 && n <= 0xff; d-- {
			n *= 2
		}
	}
	c[SeatIx(p)] = uint8(min(int(c[SeatIx(p)])+n, 255))
}

// putContract is OpExt ExtPutContract: each player Obj names gets B contract
// counters. A nonzero C marks the put as made by Miss Highwater's loss Effect
// (kop.Instr.C of the ext op, 0 in every table kgen emits until its lowering
// arms that Effect): the resolving controller then owes the player's loss copy
// (ctr_losscopy.go). With C zero no record is made, as gorge arms
// nothing for the Effect.
//
//kmanifest:ext ExtPutContract
func (s *State) putContract(k *prep, in *kop.Instr) (ok, done bool) {
	q := s.players(k, in.Obj)
	n := s.value(k, in.B)
	for i := uint8(0); i < q.n; i++ {
		s.addContract(k, q.s[i], n)
		if in.C != 0 && s.contracts(q.s[i]) != 0 {
			s.setContractOwed(q.s[i], s.res.e.ctrl) // the Effect's loss copy (ctr_losscopy.go)
		}
	}
	return true, true
}

// contractLine reports whether a carries the kernel's contract reads: a
// Tf5DmgNoContract trigger or an OpExt ExtPutContract.
func contractLine(a *kop.Ability) bool {
	if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigDamageDone && a.Trig.Flags5&kop.Tf5DmgNoContract != 0 {
		return true
	}
	for i := range a.Prog {
		if in := &a.Prog[i]; in.Op == kop.OpExt && in.Ext() == kop.ExtPutContract {
			return true
		}
	}
	return false
}
