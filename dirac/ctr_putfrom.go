package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// putCountersFrom is OpPutCountersFrom (gorge putCounterEachFromSource, the
// CounterType$ EachFromSource shape: "put its counters on target
// creature"). The source (Ref C: the triggering object of a dies trigger,
// or the source itself) is read through gorge's LKI ladder: its LIVE
// counters when it is on the battlefield holding a positive count (the
// same object re-entered, CR 400.7 notwithstanding: gorge reads g.Obj(id)
// first), else the resolving entry's LKI snapshot when it names that object
// (the trigger's pre-move snapshot, enqueueLKI; the cost-sacrifice
// snapshot, mana_window.go paySacs). Each kind's amount is its count times A
// (CounterNum$, default 1); each is put as OpPutCounter puts it
// (bolsterPut: the counter replacements, CounterAddedOnce). A Class level
// (CtrLevel) is not a counter. gorge emits one CounterChange per kind in
// the source's insertion order while the kernel puts +1/+1 first then the
// general slots ascending: two or more kinds that a counter-added trigger
// hears stall (StallCtrOrder; the order of puts no trigger hears, a stun
// beside +1/+1 counters under a +1/+1 trigger, reaches no stack or state). The recipient must be on the
// battlefield (gorge counters a non-battlefield object target too: an
// off-battlefield recipient is not representable here).
func (s *State) putCountersFrom(k *prep, in *kop.Instr) {
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		return
	}
	src := s.objRefObj(kop.Ref(in.C))
	if src == NoObj {
		return
	}
	mult := s.value(k, in.A)
	if mult <= 0 {
		return
	}
	var p1 int
	var ck [ctrSlots]uint8
	var cn [ctrSlots]uint16
	var over [maxCtrOver]ctrOverEntry
	nover := 0
	ob := &s.objs[src]
	have := false
	if ob.zone == kop.ZoneBattlefield {
		p := &s.perms[ob.slot]
		p1, ck, cn = int(p.p1), p.ck, p.cn
		have = p1 > 0
		for i := range ck {
			have = have || cn[i] > 0 && kop.CounterKind(ck[i]) != kop.CtrLevel
		}
		if p.ck[ctrSlots-1] != 0 && s.ctrOverSum(src, 0) != 0 {
			have = true
			nover = copy(over[:], s.ctrOverOf(src, 0))
		}
	}
	if !have {
		e := &s.res.e
		if e.li == 0 || s.lk(e.li).obj != src {
			return
		}
		r := s.lk(e.li)
		p1, ck, cn = int(r.p1p1), r.ck, r.cn
		nover = copy(over[:], s.ctrOverOf(src, e.li))
	}
	s.putCounterSnapshot(k, sl, p1, ck, cn, over[:nover], mult)
}

// putCounterSnapshot puts, on the permanent at sl, mult times each counter
// kind of a snapshot (the +1/+1 count, the inline slots, then the overflow
// entries; a Class level is not a counter), one bolsterPut per kind. Two or
// more kinds that a counter-added trigger hears stall (StallCtrOrder).
// putCountersFrom and copyEnteredCounters share it.
func (s *State) putCounterSnapshot(k *prep, sl Slot, p1 int, ck [ctrSlots]uint8, cn [ctrSlots]uint16, over []ctrOverEntry, mult int) {
	kinds, heard := 0, 0
	if p1 > 0 {
		kinds++
		if s.ctrAddedHeard(k, kop.CtrP1P1) {
			heard++
		}
	}
	for i := range ck {
		if cn[i] > 0 && kop.CounterKind(ck[i]) != kop.CtrLevel {
			kinds++
			if s.ctrAddedHeard(k, kop.CounterKind(ck[i])) {
				heard++
			}
		}
	}
	for _, x := range over {
		if x.n > 0 && kop.CounterKind(x.kind) != kop.CtrLevel {
			kinds++
			if s.ctrAddedHeard(k, kop.CounterKind(x.kind)) {
				heard++
			}
		}
	}
	if kinds == 0 {
		return
	}
	if heard > 1 {
		s.capFull(StallCtrOrder)
		return
	}
	if p1 > 0 {
		s.bolsterPut(k, sl, int(kop.CtrP1P1), p1*mult)
	}
	for i := range ck {
		if cn[i] > 0 && kop.CounterKind(ck[i]) != kop.CtrLevel {
			s.bolsterPut(k, sl, int(ck[i]), int(cn[i])*mult)
		}
	}
	for _, x := range over {
		if x.n > 0 && kop.CounterKind(x.kind) != kop.CtrLevel {
			s.bolsterPut(k, sl, int(x.kind), int(x.n)*mult)
		}
	}
}

// ctrAddedHeard reports whether a counter-added trigger on the battlefield
// (or an emblem) could fire for a put of counter kind c: the order of the
// puts is observable (the stack order of their triggers) only between kinds
// that are heard (CR 603.3b). The check is a superset of the real match: the
// trigger's recipient filter is not read.
func (s *State) ctrAddedHeard(k *prep, c kop.CounterKind) bool {
	const ev = kop.TrigCounterAddedOnce
	if !s.subs[ev].any() && (k.gen == nil || (k.gen.embObjEv|k.gen.grantTrigEv)&(1<<ev) == 0) {
		return false
	}
	if k.gen != nil && k.gen.embObjEv&(1<<ev) != 0 {
		return true
	}
	heard := func(a *kop.Ability) bool {
		return a.Trig.N == kop.TrigNAnyCounter || kop.CounterKind(a.Trig.N) == c
	}
	if k.gen != nil {
		found := false
		s.eachTrigger(k, ev, func(_ trigListener, a *kop.Ability) { found = found || heard(a) })
		return found
	}
	for m := s.subs[ev].and(s.occ); m.any(); {
		sl := m.first()
		m = m.without(sl)
		cd := &k.t.Cards[s.cardAt(sl)]
		for i := range cd.Abilities {
			if a := &cd.Abilities[i]; a.Kind == kop.AbTriggered && a.Trig.Event == ev && heard(a) {
				return true
			}
		}
	}
	return false
}
