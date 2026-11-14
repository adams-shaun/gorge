package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

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

// riderDeferred reports whether sl is a source whose *first* mana ability is
// a bounded rider: gorge's planner defers that alternative, so the census
// skips the source. A later rider ability beside a plain first one (Grove of
// the Burnwillows) leaves the first unit priced and is only a prefix gap
// (mana_tapsdefer.go).
func (s *State) riderDeferred(k *prep, sl Slot) bool {
	if k.gen == nil || !k.gen.anyManaRider || s.genLost(k, sl) {
		return false
	}
	c := s.cardAt(sl)
	if _, _, ok := cardManaRider(&k.t.Cards[c]); ok {
		return true // Twitching Doll's counter-on-self rider (manaCtrRider)
	}
	_, ok := s.riderAbilityOf(k, sl)
	return ok && int(k.cards[c].manaAb) == int(k.riderAbOf(c))
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

// riderPotential reports whether the source at sl's first mana ability is a
// bounded rider activation the potential walks count (gorge potentialManaFor:
// every mana ability of every untapped source, so a trigger's own cost is
// offered over it): the windowPick rider arm's condition. A rider beside a
// plain first ability keeps its first unit priced and stays a prefix gap.
func (s *State) riderPotential(k *prep, p uint8, sl Slot) bool {
	if k.gen == nil || !k.gen.anyManaRider {
		return false
	}
	c := s.cardAt(sl)
	a, ok := s.riderAbilityOf(k, sl)
	return ok && int(k.cards[c].manaAb) == int(k.riderAbOf(c)) && s.riderActivatable(k, p, sl, a)
}

// riderAbOf is card c's bounded rider ability index (prep.riderAb), -1 none.
func (k *prep) riderAbOf(c CardID) int8 {
	if int(c) >= len(k.riderAb) {
		return -1
	}
	return k.riderAb[c]
}

// riderAbilityOf is the bounded rider ability of the source at sl, when it has
// one: a mana ability whose kop.Ability.ManaRider is set (mana_rider.go).
func (s *State) riderAbilityOf(k *prep, sl Slot) (*kop.Ability, bool) {
	c := s.cardAt(sl)
	ra := k.riderAbOf(c)
	if ra < 0 || int(ra) >= len(k.t.Cards[c].Abilities) {
		return nil, false
	}
	a := &k.t.Cards[c].Abilities[ra]
	if a.Kind != kop.AbMana || a.ManaRider == kop.ManaRiderNone {
		return nil, false
	}
	return a, true
}

// riderPlainFirst reports whether the source at sl prints a plain one-mana
// tap ability before its bounded rider (Grove of the Burnwillows: "{T}: Add
// {C}" before "{T}: Add {R} or {G}. Each opponent gains 1 life"). gorge's
// prefix search activates the source once and answers its one mana ask,
// whose options are the plain ability's production and the rider's colours;
// riderPlainGap mirrors that order. Only a fixed one-mana tap is that first
// branch.
func (s *State) riderPlainFirst(k *prep, sl Slot) bool {
	a, ok := s.riderAbilityOf(k, sl)
	if !ok || a.ManaEach {
		return false
	}
	c := s.cardAt(sl)
	idx := k.cards[c].manaAb
	if idx < 0 || int(idx) == int(k.riderAbOf(c)) || int(idx) >= len(k.t.Cards[c].Abilities) {
		return false
	}
	ab := &k.t.Cards[c].Abilities[idx]
	if ab.Kind != kop.AbMana || ab.ManaRestrict != 0 || len(ab.Prog) != 0 {
		return false
	}
	pc := s.manaPCGen(k, sl)
	if pc.manaAb != idx || !pc.manaTap || pc.manaN != 1 || pc.manaChosen || pc.manaOne || pc.manaMulti || pc.manaDyn {
		return false
	}
	col := uint8(pc.manaCol)
	return col != 0 && col&(col-1) == 0
}

// riderActivatable reports whether the bounded rider ability a of the source at
// sl can be activated now: untapped (and not summoning sick when its cost taps
// a creature) and its own counters enough.
func (s *State) riderActivatable(k *prep, p uint8, sl Slot, a *kop.Ability) bool {
	if !s.occ.has(sl) || s.genLost(k, sl) || s.activationSick(k, sl) ||
		k.gen != nil && k.gen.anyCantAct && s.actForbidden(k, sl, true) {
		return false
	}
	pm := &s.perms[sl]
	if a.Cost.Flags&kop.CostTap != 0 && pm.flags&pfTapped != 0 {
		return false
	}
	if a.Cost.Flags&kop.CostRemoveCtr != 0 && pm.ctrN(s, int(a.Cost.Ctr)) < int(a.Cost.N) {
		return false
	}
	return true
}

// riderGap activates the bounded rider source at sl for p: its counters, its
// tap, its mana, then its rider (CR 605.3b). answer is the colour answer index among the
// production's bits.
func (s *State) riderGap(k *prep, p uint8, sl Slot, answer int) bool {
	a, ok := s.riderAbilityOf(k, sl)
	if !ok || !s.riderActivatable(k, p, sl, a) {
		return false
	}
	pm := &s.perms[sl]
	if a.Cost.Flags&kop.CostRemoveCtr != 0 {
		pm.seeCtr(s) // the activation path's CostRemoveCtr (mana_window.go)
		pm.subCtrN(s, int(a.Cost.Ctr), int(a.Cost.N))
	}
	if a.Cost.Flags&kop.CostTap != 0 {
		pm.flags |= pfTapped
		s.touchSl(sl)
		s.manaTapped(k, sl)
	}
	s.manaRiderFloat(k, p, sl, a, answer)
	s.manaRiderEffect(k, sl, a)
	return true
}

// riderPlainGap activates a riderPlainFirst source for the script search's
// answer c: the source's mana ask offers the plain first ability's
// production before the rider's colours (engine order), and gorge's
// scriptSearch stably reorders those answers by the play's needed pips
// (scriptOrderAnswers). c indexes that reordered list; the chosen plain
// production taps without the rider, a chosen rider production runs it.
func (s *State) riderPlainGap(k *prep, p uint8, a Action, sl Slot, c int) bool {
	if !s.riderPlainFirst(k, sl) {
		return false
	}
	ra, _ := s.riderAbilityOf(k, sl)
	var cols [kop.NumManaKinds]uint8
	var rider [kop.NumManaKinds]int
	n := 0
	pc := s.manaPCGen(k, sl)
	cols[n], rider[n] = uint8(bits.TrailingZeros8(uint8(pc.manaCol))), -1
	n++
	for col := 0; col < kop.NumManaKinds; col++ {
		if uint8(ra.ManaColors)&(1<<uint(col)) != 0 {
			cols[n], rider[n] = uint8(col), bits.OnesCount8(uint8(ra.ManaColors)&uint8((1<<uint(col))-1))
			n++
		}
	}
	need := s.playNeed(k, a)
	for i := 1; i < n; i++ { // needFirst over colours and their productions
		if !need[cols[i]] {
			continue
		}
		col, r := cols[i], rider[i]
		j := i
		for j > 0 && !need[cols[j-1]] {
			cols[j], rider[j] = cols[j-1], rider[j-1]
			j--
		}
		cols[j], rider[j] = col, r
	}
	if c >= n {
		return false
	}
	if rider[c] < 0 {
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		s.manaTapped(k, sl)
		s.tapDeferPlain(k, p, sl)
		return true
	}
	return s.riderGap(k, p, sl, rider[c])
}

// riderAnswerBit is the index of colour col among mask's bits (the script's
// answer encoding); 0 when col is not in mask.
func riderAnswerBit(mask uint8, col int) int {
	n := 0
	for c := 0; c < kop.NumManaKinds; c++ {
		if mask&(1<<uint(c)) == 0 {
			continue
		}
		if c == col {
			return n
		}
		n++
	}
	return 0
}

// manaRiderFloat adds the rider ability a's mana to p's pool: one of a chosen
// colour, each colour of an all-at-once production, or a fixed colour, its
// amount a.ManaN. answer selects the colour among the mask's bits.
func (s *State) manaRiderFloat(k *prep, p uint8, sl Slot, a *kop.Ability, answer int) {
	n := int(a.ManaN)
	if n == 0 {
		n = 1
	}
	mask := uint8(a.ManaColors)
	if a.ManaEach {
		for col := 0; col < kop.NumManaKinds; col++ {
			if mask&(1<<uint(col)) != 0 {
				s.pl[SeatIx(p)].pool[col] += uint8(n)
				s.artTag(k, p, sl, col)
			}
		}
		return
	}
	col := 0
	if mask == uint8(kop.ColAny) || a.AFlags&kop.AfManaChoice != 0 {
		i := answer
		for c := 0; c < kop.NumManaKinds; c++ {
			if mask&(1<<uint(c)) == 0 {
				continue
			}
			if i == 0 {
				col = c
				break
			}
			i--
		}
	} else if mask != 0 {
		col = bits.TrailingZeros8(mask)
	}
	s.pl[SeatIx(p)].pool[col] += uint8(n)
	s.artTag(k, p, sl, col)
}

// manaRiderEffect runs a's rider after its mana (CR 605.3b): the
// source's sacrifice once its last counter is gone, or each opponent gaining
// life.
func (s *State) manaRiderEffect(k *prep, sl Slot, a *kop.Ability) {
	switch a.ManaRider {
	case kop.ManaRiderDeplete:
		if s.perms[sl].ctrN(s, int(a.ManaRiderCtr)) == 0 {
			s.fireSac(k, sl)
			s.leaveParked(k, sl)
		}
	case kop.ManaRiderOppGainLife:
		n := int(a.ManaRiderN)
		if n < 1 {
			n = 1
		}
		q := s.oppsFrom(s.perms[sl].ctrl)
		for i := uint8(0); i < q.n; i++ {
			s.gainLife(k, q.s[i], n)
		}
	}
}
