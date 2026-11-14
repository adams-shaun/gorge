package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Contract counters (Miss Highwater): a player gets one from an OpPutCounter
// of kop.CtrContract on a player operand, and a combat DamageDone line with
// kop.Tf5DmgNoContract (ValidTarget$ Player.counters_EQ0_Contract) fires
// only on a damaged player who has none. gorge keeps the count in
// state.Player.Counters ("Contract", one PlayerCounterChange per put) and
// its proliferate offers a player holding one (effects hasRealCounters),
// so the kernel's proliferate does too.
//
// The record is a cold array of one count per seat, kept only when
// genPrep.anyContract.

// contractCold is the per-seat count, or nil when the table keeps none.
func (s *State) contractCold() []uint8 {
	l := s.cold.layout
	if l == nil || l.contract == 0 {
		return nil
	}
	at := l.beforeAtkLast()
	_ = coldView[uint8](nil, &at, l.atkLast)
	return coldView[uint8](s.cold.buf, &at, l.contract)
}

// beforeAtkLast is the byte offset just past the partner records, where the
// attack record starts: the one prefix walk atkLastCold and contractCold
// share, in the order place() lays the arrays out.
func (l *coldLayout) beforeAtkLast() int {
	at := 0
	_ = coldView[mayPlay](nil, &at, l.may)
	_ = coldView[embRec](nil, &at, l.emb)
	_ = coldView[plusCreRec](nil, &at, l.ctrP)
	_ = coldView[xblkRec](nil, &at, l.xblk)
	_ = coldView[cmdPartner](nil, &at, l.partner)
	return at
}

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

// putContract is OpPutCounter of kop.CtrContract: each player Obj names gets
// A contract counters.
func (s *State) putContract(k *prep, in *kop.Instr) {
	q := s.players(k, in.Obj)
	n := s.value(k, in.A)
	for i := uint8(0); i < q.n; i++ {
		s.addContract(k, q.s[i], n)
	}
}

// contractLine reports whether a carries the kernel's contract reads: a
// Tf5DmgNoContract trigger or a put of kop.CtrContract.
func contractLine(a *kop.Ability) bool {
	if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigDamageDone && a.Trig.Flags5&kop.Tf5DmgNoContract != 0 {
		return true
	}
	for i := range a.Prog {
		if in := &a.Prog[i]; in.Op == kop.OpPutCounter && kop.CounterKind(in.C) == kop.CtrContract {
			return true
		}
	}
	return false
}
