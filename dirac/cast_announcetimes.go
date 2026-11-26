package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Announce$ NumTimes (Tranquil Frillback: "you may pay {G} up to three
// times. When you pay this cost one or more times, choose up to that many";
// gorge cumulative.go triggeredCostNumTimesAsk and its trigger_cost_x answer,
// cr-numtimes-announce 5dc9b0697). ExtNumTimes asks the repetition count
// 0..m (ChX, mode = the count; gorge's trigger_cost_x choose) before the
// payment: m is the most repetitions up to the maximum (C>>4&7, or the open
// announce's 64, gorge's xFoldBoundCap, when F is set) whose total the
// payer's potential mana covers, found by the same census the pay ask uses
// (canPayGen). The answer rides the resolution entry's X: 0 declines,
// jumping C>>7 instructions (the whole rest of the body, as a declined
// IfPaid); a count n >= 1 leaves the IfPaid that follows to charge the unit
// n times (numTimesScale in execPay; the coloured pips scale at the
// ManaCost builds, numTimesCost) and ask pay or decline as any trigger's own
// cost. With no repetition payable nothing is asked: the IfPaid poses its
// decline-only ask on one unit, gorge's own fallback.

// numTimesOpenMax is the open announce's repetition bound: gorge's
// xFoldBoundCap (rules cumulative.go), the ceiling of "any number of times".
//
//lockstep:const engine/rules.xFoldBoundCap
const numTimesOpenMax = 64

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
	unit := int(in.C & 15)
	max := int(in.C >> 4 & 7)
	if in.F != 0 {
		max = numTimesOpenMax // no AnnounceMax$: gorge's own bound
	}
	// gorge prices the count against the payment-aware potential pool (cumulative.go
	// triggeredCostNumTimesAsk, potentialManaFor with the trigger cost as the
	// payment), an over-bound (CR 118.3 then refuses a count the sources cannot
	// fund: the payer declines). A Chosen-colour source whose colour is fixed
	// makes exactly that colour (CR 106.6); the plain bound priced it open in
	// every colour. A trigger's own cost is no spell's, an ability's or a cast's
	// (payDesc pdOther, as execTrigX). The
	// scaled cost rides kop.ManaCost's bytes; kgen refused a unit that leaves
	// one at the bound, so the build below cannot wrap.
	var av avail
	d := payDesc{class: pdOther}
	s.availPotentialFor(k, e.ctrl, &av, &d, false)
	m := 0
	for n := 1; n <= max; n++ {
		c := kop.ManaCost{Generic: uint8(unit * n)}
		for i := 0; i < 5; i++ {
			c.Pips[i] = uint8(int(in.B.N>>(3*i)&7) * n)
		}
		if !av.affords(&c, pipMaskOf(&c), 0) {
			break
		}
		m = n
	}
	if m == 0 {
		return true
	}
	s.g.pickLeft = uint8(m)
	s.res.ask, s.res.askP = raNumTimes, e.ctrl
	return false
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

// numTimesScale scales the unit cost's generic part (amt) of the pay
// instruction at the current pc by the announced count (at least 1: with no
// announcement the unit stands). The coloured pips ride the instruction's
// 3-bit packing and cannot scale in it; the ManaCost builds scale them
// (numTimesCost).
//
//go:noinline
func (s *State) numTimesScale(k *prep, amt int, pips int16) (int, int16) {
	if !s.afterNumTimes(k) {
		return amt, pips
	}
	return amt * max(int(s.res.e.x), 1), pips
}

// numTimesCost is manaOf(amt, pips) with the coloured pips scaled by the
// announced count: the generic arrives already scaled (numTimesScale in
// execPay), the unit's packed pips do not fit their 3-bit packing scaled, so
// they scale here, on the bytes. kgen refused a unit whose count past the
// bound leaves a byte, so the multiply cannot wrap. Outside an announcement
// it is manaOf.
//
//go:noinline
func (s *State) numTimesCost(k *prep, amt int, pips int16) kop.ManaCost {
	c := manaOf(amt, pips)
	if k.gen == nil || !k.gen.anyNumTimes || !s.afterNumTimes(k) {
		return c
	}
	n := max(int(s.res.e.x), 1)
	for i := range c.Pips {
		c.Pips[i] *= uint8(n)
	}
	return c
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
