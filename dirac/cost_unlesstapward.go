package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// two UnlessCost$ payments through gorge's unless walk
// (rules/pay/unlessflow.go AdvanceUnless). Cold: reached only from an
// unless tapXType pay answer and a ward trigger whose amount carries
// wardMLBit.
//
// An UnlessCost$ tapXType<N/Spec> (Command Bridge, Public Thoroughfare): the
// payer answers gorge's pay / don't-pay modes first (execPay's ordinary
// ask, unlike a trigger's own tapXType whose tap election IS the payment);
// on pay the "tapcost" part (UnlessKindTap, unlessSpecialPicks lo == hi ==
// N) is recorded unasked when exactly N untapped candidates exist, else
// picked (the kernel's raGenPayPick, one per answer).
//
// Ward—{N} and pay L life (Ovika, Enigma Goliath; Gisa, the Hellraiser):
// effWard's pay / don't-pay, then AdvanceUnless: the mana window opens while
// the whole cost (life included, CostPayableClassLife) is not payable from
// the pool and a window source remains, and PayManaConv then charges the
// mana and the life together (pay.PayManaDescriptorForSpent: the mana's
// spend, then the life). Short of the life, the window taps every source
// (the oracle's payWindowAnswer never answers done while one is left) and
// the ward counters.

// wardMLBit marks a ward trigger amount (stackEntry.trigAmt) as {N} and pay
// L life: N in bits 0-4, L in bits 5-9.
const wardMLBit int16 = 0x0800

// DebugWardMLNoLife skips the mana-and-life ward's life loss
// (TestUnlesstapwardMutation only).
var DebugWardMLNoLife bool

// unlessTapAuto records an UnlessCost$ tapXType payment whose untapped
// candidates are exactly N (gorge AdvanceUnless: RecordUnlessPaymentPick,
// no ask), tapping them in battlefield order; false = the pick is asked.
//
//go:noinline
func (s *State) unlessTapAuto(k *prep) bool {
	var d Decision
	s.resolveOptionsGen(k, &d)
	if int(d.N) != int(s.g.pickLeft) {
		return false
	}
	objs := d.Options[:d.N]
	for i := 1; i < len(objs); i++ { // battlefield (entry) order
		for j := i; j > 0; j-- {
			a, b := s.objs[objs[j-1].Source()], s.objs[objs[j].Source()]
			if s.perms[a.slot].ts <= s.perms[b.slot].ts {
				break
			}
			objs[j-1], objs[j] = objs[j], objs[j-1]
		}
	}
	for _, a := range objs {
		o := Obj(a.Source())
		s.tapCostPerm(k, s.objs[o].slot, o)
	}
	s.g.pickLeft = 0
	return true
}

// wardML is the {N} and L life of a mana-and-life ward amount.
func wardML(amt int16) (int, int) {
	return int(amt & 0x1f), int(amt >> 5 & 0x1f)
}

// resolveWardManaLife poses effWard's pay / don't-pay election.
//
//go:noinline
func (s *State) resolveWardManaLife(k *prep) {
	e := &s.res.e
	idx := s.wardedIndex(e)
	if idx < 0 {
		s.finishResolve(k, false)
		return
	}
	n, l := wardML(e.trigAmt)
	var av avail
	s.availWindowFor(k, e.trigPl, &av)
	c := kop.ManaCost{Generic: uint8(n)}
	if av.affords(&c, 0, 0) && int(s.pl[e.trigPl].life) >= l || k.gen != nil {
		s.res.ask = raGeneric // pay (1) or not (0)
		s.res.askP = e.trigPl
		s.res.left = 1
		s.ctl = ctlResolve
		return
	}
	s.wardCounterAt(k, idx)
	s.finishResolve(k, false)
}

// applyWardManaLife answers it: paid, the mana window then the life; short
// of either, the window taps what it has and the warded entry is countered.
//
//go:noinline
func (s *State) applyWardManaLife(k *prep, a Action) {
	r := &s.res
	idx := s.wardedIndex(&r.e)
	n, l := wardML(r.e.trigAmt)
	p := r.e.trigPl
	c := kop.ManaCost{Generic: uint8(n)}
	if a.Mode() == 1 && int(s.pl[p].life) >= l && (k.gen == nil || s.affordsFor(k, p, &c) || s.windowAffords(k, p, &c)) {
		if _, short := s.payWindow(k, p, &c); short {
			if idx >= 0 {
				s.wardCounterAt(k, idx) // the colours taken left the mana unpaid, so the life is not
			}
		} else if !DebugWardMLNoLife {
			s.loseLife(k, p, l)
		}
	} else if idx >= 0 {
		if a.Mode() == 1 {
			s.tapAllForMana(k, p) // gorge's window taps what it has, then can't pay
		}
		s.wardCounterAt(k, idx)
	}
	s.finishResolve(k, false)
}
