package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Announce$ NumTimes (Tranquil Frillback: "you may pay {G} up to three
// times. When you pay this cost one or more times, choose up to that many";
// gorge cumulative.go triggeredCostNumTimesAsk and its trigger_cost_x answer,
// cr-numtimes-announce 5dc9b0697). ExtNumTimes asks the repetition count
// 0..m (ChX, mode = the count; gorge's trigger_cost_x choose) before the
// payment: m is the most repetitions up to the maximum (C>>4&7) whose total
// the payer's potential mana covers, found by the same census the pay ask
// uses (canPayGen). The answer rides the resolution entry's X: 0 declines,
// jumping C>>7 instructions (the whole rest of the body, as a declined
// IfPaid); a count n >= 1 leaves the IfPaid that follows to charge the unit
// n times (numTimesScale in execPay, the answer site and FeatPayManual) and
// ask pay or decline as any trigger's own cost. With no repetition payable
// nothing is asked: the IfPaid poses its decline-only ask on one unit,
// gorge's own fallback.

// raNumTimes is the announcement's resolution ask.
const raNumTimes uint8 = 253

// prepNumTimes sets anyNumTimes.
func prepNumTimes(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Ext() == kop.ExtNumTimes {
					g.anyNumTimes = true
				}
			}
		}
	}
}

// numTimesStart is ExtNumTimes; false = suspended on the announcement.
//
//go:noinline
func (s *State) numTimesStart(k *prep, in *kop.Instr) bool {
	e := &s.res.e
	e.x = 0
	unit, max := int(in.C&15), int(in.C>>4&7)
	// gorge prices the count against the potential pool, an over-bound (CR 118.3
	// then refuses a count the sources cannot fund: the payer declines)
	var av avail
	s.availPotential(k, e.ctrl, &av)
	affords := func(n int) bool {
		c := manaOf(unit*n, scalePips(in.B.N, n))
		return av.affords(&c, pipMaskOf(&c), 0)
	}
	m := 0
	for n := 1; n <= max; n++ {
		if !affords(n) {
			break
		}
		m = n
	}
	if in.F != 0 && m == max && affords(max+1) {
		s.capFull(StallNumTimes) // gorge offers every payable count (to 64): the kernel holds seven
	}
	if m == 0 {
		return true
	}
	s.g.pickLeft = uint8(m)
	s.res.ask, s.res.askP = raNumTimes, e.ctrl
	return false
}

// scalePips multiplies each 3-bit pip count of p by n (kgen keeps every
// count times its maximum within 7).
func scalePips(p int16, n int) int16 {
	var out int16
	for i := 0; i < 5; i++ {
		out |= int16((int(p)>>(3*i)&7)*n) << (3 * i)
	}
	return out
}

// afterNumTimes reports whether the instruction before the current one is
// ExtNumTimes.
func (s *State) afterNumTimes(k *prep) bool {
	pc := s.res.pc
	if k.gen == nil || !k.gen.anyNumTimes || pc == 0 {
		return false
	}
	return s.prog(k)[pc-1].Ext() == kop.ExtNumTimes
}

// numTimesScale scales the unit cost (generic amt, packed pips) of the pay
// instruction at the current pc by the announced count (at least 1: with no
// announcement the unit stands).
//
//go:noinline
func (s *State) numTimesScale(k *prep, amt int, pips int16) (int, int16) {
	if !s.afterNumTimes(k) {
		return amt, pips
	}
	n := max(int(s.res.e.x), 1)
	return amt * n, scalePips(pips, n)
}

// numTimesPips is numTimesScale for the pips alone (the answer sites).
func (s *State) numTimesPips(k *prep, pips int16) int16 {
	if k.gen == nil || !k.gen.anyNumTimes {
		return pips
	}
	_, p := s.numTimesScale(k, 0, pips)
	return p
}

// askOptionsNumTimes builds the announcement (the tail of askOptionsColGrant).
//
//go:noinline
func (s *State) askOptionsNumTimes(k *prep, d *Decision) bool {
	if s.res.ask != raNumTimes {
		return s.askOptionsTrigX(k, d) // trig_maypayx.go
	}
	d.Sub = ChX
	for v := 0; v <= int(s.g.pickLeft); v++ {
		d.add(MakeAction(AkChoose, 0, 0, uint8(v)))
	}
	return true
}

// applyAskNumTimes keeps the announced count.
//
//go:noinline
func (s *State) applyAskNumTimes(k *prep, a Action) bool {
	if s.res.ask != raNumTimes {
		return s.applyAskTrigX(k, a) // trig_maypayx.go
	}
	n := a.Mode()
	s.res.e.x = objWord(n)
	if n == 0 {
		s.continueRes(k, uint8(s.prog(k)[s.res.pc].C>>7))
		return true
	}
	s.continueRes(k, 0)
	return true
}
