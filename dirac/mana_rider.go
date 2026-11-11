package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A mana ability with a counter rider (Twitching Doll: "{T}: Add one mana of
// any color. Put a nest counter on this"; kgen ctrRiderMana). CR 605.3b: the
// rider resolves with the mana, at activation, with no stack and no decision.
//
// gorge's payment planner defers the source (pay.PaymentPlanAbilityTier
// classifies a tap-only ability with an unvetted SubAbility$ chain
// "source:rider"), so the census never lists it and autopay never taps it;
// the gap is relaxable (paymentPlanRelaxedAlternatives: one mana per colour,
// once), so a potential play's prefix search activates it, branching on the
// colour, and the manual window's bot never picks it. That is exactly the
// kernel's Taps-deferred route for a plain one-mana-of-any-colour source
// (mana_tapsdefer.go: the planner's first pass skips it, scriptTapsDefer makes
// it a prefix gap), so the source joins that route through planDeferred,
// statically rather than through a live Taps watcher. The window-census
// question (act_waterbend.go windowCensusComplete, gorge
// manaProductionAltered) reads only interference and keeps tapsDeferred.
//
// The rider runs wherever the kernel taps the source for mana (manaTapped):
// the counters go through the counter replacements and the counter-added
// triggers as OpPutCounter's do. Two counter replacements of mixed kinds would
// pose gorge's CR 616.1 order ask inside the activation, which the kernel
// cannot pose mid-payment: the game stalls (StallCtrOrder) instead.

// manaCtrRider reads a's counter rider: its program is exactly "put N
// counters of kind on this permanent" (kgen ctrRiderMana).
func manaCtrRider(a *kop.Ability) (kind, n int, ok bool) {
	if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 || len(a.Prog) != 1 {
		return 0, 0, false
	}
	in := &a.Prog[0]
	if in.Op != kop.OpPutCounter || in.Obj != kop.RefSelf || in.A.K != kop.VConst || in.A.N < 1 || in.B != (kop.ValueRef{}) || in.F != 0 {
		return 0, 0, false
	}
	return int(in.C), int(in.A.N), true
}

// cardManaRider reads card c's counter rider off its first printed mana
// ability (kgen refuses the rider beside a second one).
func cardManaRider(cd *kop.CardDef) (kind, n int, ok bool) {
	j := firstManaAb(cd)
	if j < 0 {
		return 0, 0, false
	}
	return manaCtrRider(&cd.Abilities[j])
}

// riderDeferred reports whether sl is a counter-rider mana source the
// planner defers.
func (s *State) riderDeferred(k *prep, sl Slot) bool {
	if k.gen == nil || !k.gen.anyManaRider || s.genLost(k, sl) {
		return false
	}
	_, _, ok := cardManaRider(&k.t.Cards[s.cardAt(sl)])
	return ok
}

// planDeferred reports whether gorge's planner tier defers sl: a counter
// rider, a Condition-split source (mana_condsplit.go), or a live Defer-class
// Taps watcher (tapsDeferred).
func (s *State) planDeferred(k *prep, p uint8, sl Slot) bool {
	return s.riderDeferred(k, sl) || s.splitDeferred(k, sl) || s.tapsDeferred(k, p, sl)
}

// deferLive reports whether some source may be planner-deferred now: the
// gate the planner and the walks lift out of their loops.
func (s *State) deferLive(k *prep) bool {
	return k.gen != nil && (k.gen.anyManaRider || k.gen.anyCondSplit || k.gen.anyTapsDefer && s.subs[kop.TrigTaps].any())
}

// manaRider runs the counter rider of the source at sl, just tapped for its
// mana (manaTapped).
func (s *State) manaRider(k *prep, sl Slot) {
	if !s.riderDeferred(k, sl) {
		return
	}
	kind, n, _ := cardManaRider(&k.t.Cards[s.cardAt(sl)])
	if ro := &s.g.ro; k.gen.anyRepl&(rbCtrOn|rbPlusCtrEOT) != 0 && ro.n == 0 && !s.putCounterBlocked(k, sl, kind) {
		if c := s.ctrGather(k, sl, kind); c.ok && c.n == 2 && c.mixed() {
			s.capFull(StallCtrOrder) // gorge's CR 616.1 order ask inside the activation
			return
		}
	}
	s.g.lastCK = kop.CounterKind(kind) // CounterAddedOnce reads the kind
	m := s.placeCounters(k, sl, kind, n, s.perms[sl].ctrl, ctrPutEffect)
	if m > 0 && k.anyCtrAdded {
		s.ctrNote(k, sl, kind)
	}
	if m > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
		s.fireObj(k, kop.TrigCounterAddedOnce, sl, s.perms[sl].obj, s.perms[sl].ctrl, int16(m))
	}
}
