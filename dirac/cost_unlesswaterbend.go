package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An UnlessCost$ Waterbend<N> (Waterbending Lesson; CR 701.67a). Gorge
// (rules/pay/unlessflow.go AdvanceUnless, unlessspecial.go): the payer
// answers the pay / don't-pay modes, then the taps owed are N less what the
// floating pool covers (WaterbendTapsOwed); owing none, or exactly as many
// candidates as are owed, nothing is asked, else one pick of that many
// untapped artifacts and creatures the payer controls. PayWaterbend then
// pays the rest of the {N} from the pool and taps the picks. A short supply
// of candidates declines. It rides PayTapX with B.N PayBSubWaterbend (Instr.C's
// nibble is full); execPay widens it to payKindWaterbend.
//
// The pick is asked one permanent per answer (the kernel's multi-pick idiom,
// as the unless tapXType), each tapped as it is chosen; the pool is charged
// when the last is in. Cold: reached only from that sentinel.

// Ward—Waterbend {N} (The Unagi of Kyoshi Island; gorge rules/ward.go
// beginWardPayment): the pay / don't-pay election (ChWardPay), then no taps
// owed pays the pool unasked, fewer candidates than owed counters, else
// ward_waterbend asks the owed count even when exactly that many exist
// (askWardObjects has no auto-record). The ward trigger's amount carries
// wardWBBit | N.

// wardWBBit marks a ward trigger amount (stackEntry.trigAmt) as Waterbend N
// in bits 0-4. 0x0200 is also the top bit of a mana-and-life ward's life, so
// it only counts while wardMLBit is clear.
const wardWBBit int16 = 0x0200

// ChWardWaterbend is gorge's "ward_waterbend" pick: source = the permanent.
const ChWardWaterbend ChoiceKind = 247

// payKindWaterbend is the pay kind (execPay's kind, pickZ) of an unless
// Waterbend<N>.
const payKindWaterbend = 21

// DebugUnlessWaterbendNoPool charges the whole {N} as taps, ignoring the
// floating pool (mutation).
var DebugUnlessWaterbendNoPool bool

// unlessPayKind widens a pay instruction's kind nibble: a PayTapX whose B.N
// is PayBSubWaterbend is an unless Waterbend<N> (execPay, branchPayable).
func unlessPayKind(kind int, bn int16) int {
	if kind == kop.PayTapX && bn == kop.PayBSubWaterbend {
		return payKindWaterbend
	}
	return beholdPayKind(kind, bn) // a resolution Behold<1/Type> rides PayDiscard (cost_unlessbehold.go)
}

// unlessWaterbendOwed is how many permanents p must tap to waterbend n.
func (s *State) unlessWaterbendOwed(p uint8, n int) int {
	if DebugUnlessWaterbendNoPool {
		return n
	}
	return max(n-poolTotal(&s.pl[p&1]), 0)
}

// unlessWaterbendCand reports whether sl can be tapped for the waterbend:
// untapped, an artifact or a creature (gorge unlessSpecialCandidates).
func (s *State) unlessWaterbendCand(k *prep, sl Slot) bool {
	return s.perms[sl].flags&pfTapped == 0 && s.typesAt(k, sl)&(kop.TArtifact|kop.TCreature) != 0
}

// unlessWaterbendCands counts p's candidates, calling each for every one in
// battlefield (ctrlM) order.
func (s *State) unlessWaterbendCands(k *prep, p uint8, each func(Slot)) int {
	n := 0
	for m := s.ctrlM[p&1]; m.any(); m = m.rest() {
		if sl := m.first(); s.unlessWaterbendCand(k, sl) {
			if each != nil {
				each(sl)
			}
			n++
		}
	}
	return n
}

// unlessWaterbendPayable is canPayGen's waterbend arm.
func (s *State) unlessWaterbendPayable(k *prep, p uint8, n int) bool {
	return s.unlessWaterbendCands(k, p, nil) >= s.unlessWaterbendOwed(p, n)
}

// startUnlessWaterbend answers "pay": the picks are asked unless none or all
// the candidates are owed; false = a pick is pending.
//
//go:noinline
func (s *State) startUnlessWaterbend(k *prep) bool {
	p, n := s.g.pickP, int(s.g.pickLeft)
	owed := s.unlessWaterbendOwed(p, n)
	have := s.unlessWaterbendCands(k, p, nil)
	if have < owed {
		s.payDone(k, false)
		return false
	}
	ward := s.g.pickF&pfWard != 0
	if owed == 0 {
		s.payWaterbendPool(p, n, 0) // the pool covers it: nothing is tapped (gorge WaterbendTapsOwed 0, ward need 0)
		s.payDone(k, true)
		return false
	}
	if have == owed && !ward {
		s.unlessWaterbendCands(k, p, func(sl Slot) { s.tapCostPerm(k, sl, s.perms[sl].obj) })
		s.payWaterbendPool(p, n, owed)
		s.payDone(k, true)
		return false
	}
	s.g.pickLeft = uint8(owed)
	s.res.ask, s.res.askP = raGenPayPick, p
	return true
}

// payWaterbendPool charges the {N} the taps did not cover from the pool.
func (s *State) payWaterbendPool(p uint8, n, taps int) {
	if rest := n - taps; rest > 0 {
		c := kop.ManaCost{Generic: uint8(rest)}
		s.payFromPool(p, &c, 0)
	}
}

// resolveWardWaterbend poses the ward's pay / don't-pay election.
//
//go:noinline
func (s *State) resolveWardWaterbend(k *prep) {
	e := &s.res.e
	if s.wardedIndex(e) < 0 {
		s.finishResolve(k, false)
		return
	}
	s.g.pickF = pfWard
	s.g.pickZ = payKindWaterbend
	s.g.pickLeft = uint8(e.trigAmt &^ wardWBBit)
	s.g.pickP = e.trigPl
	s.res.ask, s.res.askP = raGenPay, e.trigPl
	s.ctl = ctlResolve
}

// unlessWaterbendOptions poses the pick: every untapped candidate.
func (s *State) unlessWaterbendOptions(k *prep, d *Decision) {
	d.Sub = ChWaterbend
	if s.g.pickF&pfWard != 0 {
		d.Sub = ChWardWaterbend
	}
	s.unlessWaterbendCands(k, s.g.pickP, func(sl Slot) {
		d.add(MakeAction(AkChoose, uint8(s.perms[sl].obj), 0, 0))
	})
}

// applyUnlessWaterbendPick taps the pick; the last one pays the pool's share.
func (s *State) applyUnlessWaterbendPick(k *prep, o Obj) {
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield {
		s.tapCostPerm(k, ob.slot, o)
	}
	s.g.pickLeft--
	if s.g.pickLeft > 0 {
		return
	}
	n := int(s.res.e.trigAmt &^ wardWBBit) // a ward's N
	if s.g.pickF&pfWard == 0 {
		n = int(s.value(k, s.prog(k)[s.res.pc].A))
	}
	s.payWaterbendPool(s.g.pickP, n, s.unlessWaterbendOwed(s.g.pickP, n))
	s.payDone(k, true)
}
