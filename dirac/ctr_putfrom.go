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
	var ck [ctrSlots]uint16
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
func (s *State) putCounterSnapshot(k *prep, sl Slot, p1 int, ck [ctrSlots]uint16, cn [ctrSlots]uint16, over []ctrOverEntry, mult int) {
	kinds, heard := 0, 0
	var heardKinds [ctrSlots + 1 + maxCtrOver]kop.CounterKind
	nh := 0
	note := func(c kop.CounterKind, n int) {
		if n <= 0 || c == kop.CtrLevel {
			return
		}
		kinds++
		if s.ctrAddedHeard(k, c, sl) {
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
	if heard > 1 && s.ctrOrderObservable(k, heardKinds[:nh], sl) {
		s.capFull(StallCtrOrder)
		return
	}
	ctrOld := s.ctrAllBatchOpen() // one action across the kinds (trig_counteraddedall.go)
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
	s.ctrAllBatchClose(ctrOld)
}

// ctrAddedHeard reports whether a counter-added trigger on the battlefield
// (or an emblem) could fire for a put of counter kind c on the permanent at
// at: the order of the puts is observable (the stack order of their triggers)
// only between kinds that are heard (CR 603.3b). A trigger's recipient filter
// and self flag are read when at is known and the filter cannot change under
// the counters; an unknown recipient (NoSlot) is a superset, as before.
func (s *State) ctrAddedHeard(k *prep, c kop.CounterKind, at Slot) bool {
	n, _, _ := s.ctrAddedListener(k, c, at)
	return n > 0
}

// ctrAddedListener counts the listeners a put of counter kind c has, and
// returns the only one when it has exactly one.
func (s *State) ctrAddedListener(k *prep, c kop.CounterKind, at Slot) (int, trigListener, *kop.Ability) {
	return s.ctrAddedListenerBy(k, c, noPutter, at)
}

// noPutter: the put's player is unknown, so no listener is ruled out by it.
const noPutter = MaxSeats

// ctrRecipientFilterStable reports whether a counter-added trigger's
// event-object filter can be read once for the whole put: control, owner,
// type, subtype and colour selectors do not change as counters land, while a
// P/T, keyword, counter or state predicate can (a +1/+1 counter raises power,
// a keyword counter adds a keyword), so those keep the old superset.
func ctrRecipientFilterStable(k *prep, f kop.FilterID) bool {
	for n := 0; f != 0 && int(f) < len(k.t.Filters) && n < 64; n++ {
		fl := &k.t.Filters[f]
		if fl.Pred != 0 || fl.PredN != 0 || fl.Flags != 0 || fl.Flags2 != 0 ||
			fl.CtrlTgt != 0 || fl.Special != 0 || fl.PickDistinctPower || fl.PickSumMax != (kop.ValueRef{}) {
			return false
		}
		f = fl.Or
	}
	return true
}

// ctrAddedListenerBy is ctrAddedListener for a put by player putter on the
// permanent at at (NoSlot: recipient unknown). A listener whose ValidSource$
// (Whose) excludes the putter does not hear it (a "whenever you put" line on
// the other seat's permanent; CR 603.2), and one whose event-object filter
// (or TfSelf) excludes at does not hear it either (Generous Patron's "on a
// creature you don't control").
func (s *State) ctrAddedListenerBy(k *prep, c kop.CounterKind, putter uint8, at Slot) (int, trigListener, *kop.Ability) {
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
	atOK := func(l trigListener, a *kop.Ability) bool {
		if at == NoSlot {
			return true
		}
		if a.Trig.Flags&kop.TfSelf != 0 {
			return l.sl == at
		}
		if a.Trig.Filter == 0 {
			return true
		}
		if !ctrRecipientFilterStable(k, a.Trig.Filter) {
			return true // a predicate the counters can change: keep the superset
		}
		return s.matchPerm(k, a.Trig.Filter, at, l.ctrl, l.sl)
	}
	take := func(l trigListener, a *kop.Ability) {
		if !heard(a) || !atOK(l, a) || (putter != noPutter && !whoseOK(a.Trig.Whose, l.ctrl, putter)) {
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
				take(trigListener{sl: sl, ctrl: s.perms[sl].ctrl}, a)
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
func (s *State) ctrOrderObservable(k *prep, kinds []kop.CounterKind, at Slot) bool {
	return s.ctrOrderObservableBy(k, kinds, noPutter, at)
}

// ctrOrderObservableBy is ctrOrderObservable for puts by player putter on the
// permanent at at (NoSlot: recipient unknown).
func (s *State) ctrOrderObservableBy(k *prep, kinds []kop.CounterKind, putter uint8, at Slot) bool {
	var sole trigListener
	have := false
	for _, c := range kinds {
		n, l, a := s.ctrAddedListenerBy(k, c, putter, at)
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
// orders reach the same stack. A CR 603.8 state trigger (Tf4StateCtr) is the
// same: it queues one instance at the first put that meets its threshold and
// the later puts find it outstanding, whichever kind each was.
func ctrOrderInsensitive(a *kop.Ability) bool {
	g := a.Gate.Ctr
	return a.Trig.N == kop.TrigNAnyCounter && (a.Trig.Flags&kop.TfOncePerTurn != 0 || a.Trig.Flags4&kop.Tf4StateCtr != 0) &&
		(g == 0 || g == kop.CtrLevel || g == kop.CtrMaxSpeed || g == kop.CtrMode)
}
