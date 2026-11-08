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
// hears stall (StallCtrOrder) unless one once-per-turn any-kind listener
// hears all of them (ctrOrderObservable; the order of puts no trigger hears,
// a stun beside +1/+1 counters under a +1/+1 trigger, reaches no stack or
// state). The recipient must be on the
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
// more kinds that a counter-added trigger hears stall (StallCtrOrder) unless
// ctrOrderObservable proves their order cannot reach the stack.
// putCountersFrom and copyEnteredCounters share it.
func (s *State) putCounterSnapshot(k *prep, sl Slot, p1 int, ck [ctrSlots]uint8, cn [ctrSlots]uint16, over []ctrOverEntry, mult int) {
	kinds, heard := 0, 0
	var heardKinds [ctrSlots + 1 + maxCtrOver]kop.CounterKind
	nh := 0
	note := func(c kop.CounterKind, n int) {
		if n <= 0 || c == kop.CtrLevel {
			return
		}
		kinds++
		if s.ctrAddedHeard(k, c) {
			heard++
			heardKinds[nh] = c
			nh++
		}
	}
	note(kop.CtrP1P1, p1)
	for i := range ck {
		note(kop.CounterKind(ck[i]), int(cn[i]))
	}
	for _, x := range over {
		note(kop.CounterKind(x.kind), int(x.n))
	}
	if kinds == 0 {
		return
	}
	if heard > 1 && s.ctrOrderObservable(k, heardKinds[:nh]) {
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
	n, _, _ := s.ctrAddedListener(k, c)
	return n > 0
}

// ctrAddedListener counts the listeners a put of counter kind c has, and
// returns the only one when it has exactly one.
func (s *State) ctrAddedListener(k *prep, c kop.CounterKind) (int, trigListener, *kop.Ability) {
	const ev = kop.TrigCounterAddedOnce
	if !s.subs[ev].any() && (k.gen == nil || (k.gen.embObjEv|k.gen.grantTrigEv)&(1<<ev) == 0) {
		return 0, trigListener{}, nil
	}
	n := 0
	var one trigListener
	var oneA *kop.Ability
	if k.gen != nil && k.gen.embObjEv&(1<<ev) != 0 {
		n++ // an emblem hears every kind; it has no listener identity
	}
	heard := func(a *kop.Ability) bool {
		return a.Trig.N == kop.TrigNAnyCounter || kop.CounterKind(a.Trig.N) == c
	}
	take := func(l trigListener, a *kop.Ability) {
		if !heard(a) {
			return
		}
		n++
		one, oneA = l, a
	}
	if k.gen != nil {
		s.eachTrigger(k, ev, take)
		return n, one, oneA
	}
	for m := s.subs[ev].and(s.occ); m.any(); {
		sl := m.first()
		m = m.without(sl)
		cd := &k.t.Cards[s.cardAt(sl)]
		for i := range cd.Abilities {
			if a := &cd.Abilities[i]; a.Kind == kop.AbTriggered && a.Trig.Event == ev {
				take(trigListener{sl: sl}, a)
			}
		}
	}
	return n, one, oneA
}

// ctrOrderObservable reports whether the per-kind order of a putCountersFrom
// copy (the source's insertion order in gorge, p1 then the slots ascending in
// the kernel) can reach a different stack or state. It cannot when every
// heard kind is heard by the same single listener and that listener is an
// any-kind, non-counter-gated, once-per-turn CounterAdded trigger: the first
// put fires that one trigger, the rest hit its once-per-turn limit, and the
// same ability fires whichever kind came first (CR 603.3b). A kind heard by
// no listener is skipped: its position reaches no trigger.
func (s *State) ctrOrderObservable(k *prep, kinds []kop.CounterKind) bool {
	var sole trigListener
	have := false
	for _, c := range kinds {
		n, l, a := s.ctrAddedListener(k, c)
		if n == 0 {
			continue
		}
		if n != 1 || a == nil || !ctrOrderInsensitive(a) {
			return true
		}
		if have && l != sole {
			return true
		}
		sole, have = l, true
	}
	return false
}

// ctrOrderInsensitive reports whether a CounterAdded trigger can fire at most
// once per turn for every kind, with the same ability whatever kind fired it:
// an any-kind trigger with no counter gate (the class, speed and mode gates
// are not counter kinds). No effect reads the counter kind of a
// CounterAddedOnce event (only the trigger's kind filter does), so the two
// orders reach the same stack.
func ctrOrderInsensitive(a *kop.Ability) bool {
	g := a.Gate.Ctr
	return a.Trig.N == kop.TrigNAnyCounter && a.Trig.Flags&kop.TfOncePerTurn != 0 &&
		(g == 0 || g == kop.CtrLevel || g == kop.CtrMaxSpeed || g == kop.CtrMode)
}
