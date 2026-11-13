package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Characteristics on a generated table, in this fixed order (the plan's
// override record standing in for CR 613's layers): the card (or the core's
// copy-of), characteristic-defining P/T, the permanent's override records
// in application order (OpOverride), then override statics (StatOverride:
// Witness Protection, Imprisoned in the Moon), then counters, until-EOT
// deltas and Continuous statics. Keyword- and type-reading filters inside a
// static are evaluated against the layer below it.

// objBits is a set of objects.
type objBits [ObjWords]uint64

func (b *objBits) set(o Obj)      { b[o/64] |= 1 << (o % 64) }
func (b *objBits) clr(o Obj)      { b[o/64] &^= 1 << (o % 64) }
func (b *objBits) has(o Obj) bool { return b[o/64]&(1<<(o%64)) != 0 }

// statSrc is one static ability in force: on a permanent (sl) or an
// emblem (sl == NoSlot). emb is the emblem's handle (player<<4 | index+1,
// 0 = not an emblem), the key into State.g.embN.
type statSrc struct {
	a    *kop.Ability
	sl   Slot
	ctrl uint8
	obj  Obj
	emb  uint8
}

// eachStatic calls f for every static ability in force (battlefield
// permanents that haven't lost their abilities -- a lost one keeps only a
// static its own live override grant switches on -- then emblems); f returns
// false to stop.
func (s *State) eachStatic(k *prep, f func(st statSrc) bool) {
	for m := s.statM; m.any(); m = m.rest() {
		sl := m.first()
		lost := s.genLost(k, sl)
		p := &s.perms[sl]
		cd := &k.t.Cards[s.cardAt(sl)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if lost && !s.ovGateKeptOnLoss(k, sl, a) {
				continue // only a static its own live, not-older grant switches on outlives the loss
			}
			if a.Kind == kop.AbStatic || a.Kind == kop.AbReplacement {
				if !f(statSrc{a: a, sl: sl, ctrl: p.ctrl, obj: p.obj}) {
					return
				}
			}
		}
	}
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		for i := 0; i < int(s.g.nemb[pp]); i++ {
			e := &k.t.Emblems[s.embIdx(pp, i)-1]
			sl, obj := NoSlot, NoObj
			if e.HostBound() {
				// a host-bound emblem reads its host as the source
				// (filter_combatwords.go)
				if obj = s.embObj(pp, i); s.objs[obj].zone != kop.ZoneBattlefield {
					continue
				}
				sl = s.objs[obj].slot
			}
			for j := range e.Abilities {
				a := &e.Abilities[j]
				if a.Kind == kop.AbStatic {
					if !f(statSrc{a: a, sl: sl, ctrl: pp, obj: obj, emb: pp<<4 | uint8(i+1)}) {
						return
					}
				}
			}
		}
	}
}

// statCond reports whether a static's condition holds.
func (s *State) statCond(k *prep, st statSrc) bool {
	if g := st.a.Static.OvGate; g != 0 && !s.ovGranted(k, st.sl, g) {
		return false
	}
	c := st.a.Static.Cond2
	if st.a.Kind == kop.AbReplacement {
		c = st.a.Repl.Cond
	}
	if c == 0 {
		return true
	}
	return s.condEval(k, c, s.ctxStatic(st))
}

// ctxStatic is the evaluation context of a static ability.
func (s *State) ctxStatic(st statSrc) evalCtx {
	return evalCtx{src: st.obj, srcSl: st.sl, ctrl: st.ctrl}
}

// tick advances the layer clock (the one perm.ts reads) for a one-shot
// continuous effect, as gorge's AddContinuous ticks its game clock, and
// returns the effect's timestamp.
func (s *State) tick() uint16 {
	s.ts++
	return s.ts
}

// lossAt reports whether the permanent at sl has lost all abilities and the
// timestamp of the latest such effect applying to it (CR 613.1f): a one-shot
// loss, a lose-all override record, an attached Aura's lose-all static
// (whose timestamp is its source's, as gorge stamps a static effect with its
// source's entry). An ability another effect grants sl survives the loss
// only when that grant is not older (layer 6 in timestamp order, a tie
// applying the removal first; gorge
// abilityLoss and grantLostTo: a removal newer than the grant removes it).
func (s *State) lossAt(k *prep, sl Slot) (ts uint16, lost bool) {
	if k.gen.fxNeed&fxNeedL6 != 0 {
		return s.loss6(k, sl)
	}
	p := &s.perms[sl]
	if p.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
		ts, lost = s.lossFxTs(sl), true
	}
	if k.gen.anyLoseRec && s.fxM.has(sl) {
		for i := 0; i < int(s.g.nfx); i++ {
			if r := &s.g.fx[i]; r.sl == sl && r.kind == fxOverride && k.t.Overrides[r.r.idx-1].LoseAbilities {
				lost = true
				ts = max(ts, r.r.ts)
			}
		}
	}
	if k.gen.anyLoseStat {
		for m := s.statM; m.any(); m = m.rest() {
			src := m.first()
			if src == sl || s.perms[src].attached != sl || !k.gen.has(s.cardAt(src), gfLoseStat) || s.perms[src].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
				continue
			}
			lost = true
			ts = max(ts, s.layerTs(src))
		}
	}
	if lost && p.ovr.walk&walkOn != 0 {
		// a walk slot: each static's own filter and layer-4 verdict
		// decide (no loss here comes from a global static)
		return s.loss6(k, sl)
	}
	return ts, lost
}

// grantTs is the layer timestamp of a static ability's grant: its source
// permanent's, or when the emblem statSrc names, the effect's CR 613.7
// timestamp recorded when it was created (State.g.embT and the cold records). An emblem is a
// continuous effect created at resolution (gorge's AddContinuous ClockTick),
// so it is newer than every effect that was already on the battlefield when
// it entered -- and older than one that comes later.
func (s *State) grantTs(st statSrc) uint16 {
	if st.sl != NoSlot {
		return s.layerTs(st.sl)
	}
	if st.emb != 0 {
		if p, i := st.emb>>4, int(st.emb&15); p < MaxSeats && i > 0 && i <= int(s.g.nemb[p]) {
			return s.embStamp(p, i-1)
		}
	}
	return 0
}

// genLost reports whether the permanent at sl has lost all abilities, or is
// face down (CR 708.2: a face-down permanent has no abilities).
func (s *State) genLost(k *prep, sl Slot) bool {
	// ovrLoses' gates inline (kperf-blb): a slot without an override
	// record, in a table without a lose-all static or a layer-6 walk,
	// asks nothing more; a face-down table sets lossSlow (lostSlow)
	if s.perms[sl].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
		return true
	}
	return k.gen != nil && (k.gen.lossSlow || k.gen.anyLoseRec && s.fxM.has(sl)) && s.lostSlow(k, sl)
}

// loseAllOnset: an effect making the permanent at dst lose all its
// abilities starts now (src the Aura whose static applies it, NoSlot for a
// one-shot effect). The later layer-6 effect removes the until-end-of-turn
// keyword grants made before it (CR 613.7) while it applies; grants made
// after it survive. The older grants are set aside (suppressEOT) and come
// back if the loss ends the same turn (gorge's layer-6 walk; pair 42681).
func (s *State) loseAllOnset(k *prep, src, dst Slot) {
	if dst == NoSlot {
		return
	}
	if src != NoSlot {
		if k.gen == nil || !k.gen.anyOvrStat || !k.gen.has(s.cardAt(src), gfOvrStat) {
			return
		}
		lose := false
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOverride && a.Static.Override > 0 && k.t.Overrides[a.Static.Override-1].LoseAbilities {
				lose = true
			}
		}
		if !lose {
			return
		}
	}
	s.suppressEOT(dst)
	s.perms[dst].ovr.kw = 0 // an Animate's keywords are an earlier one-shot grant too
}

// ovrLoses: an override record (own or static) removes sl's abilities.
func (s *State) ovrLoses(k *prep, sl Slot) bool {
	if k.gen.fxNeed&fxNeedL6 != 0 {
		return s.loss6Lost(k, sl)
	}
	if k.gen.anyLoseRec && s.fxM.has(sl) {
		for i := 0; i < int(s.g.nfx); i++ {
			if r := &s.g.fx[i]; r.sl == sl && r.kind == fxOverride && k.t.Overrides[r.r.idx-1].LoseAbilities {
				return s.perms[sl].ovr.walk&walkOn == 0 || s.loss6Lost(k, sl)
			}
		}
	}
	if !k.gen.anyLoseStat {
		return false
	}
	// an attached Aura's lose-all static (eachOverrideStatic's sources)
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		if src == sl || s.perms[src].attached != sl || !k.gen.has(s.cardAt(src), gfLoseStat) || s.perms[src].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
			continue
		}
		return s.perms[sl].ovr.walk&walkOn == 0 || s.loss6Lost(k, sl)
	}
	return false
}

// eachOverrideStatic calls f for every StatOverride applying to sl. Its
// sources are the permanents attached to sl (the pool's override statics
// are Auras' "enchanted creature"), checked without consulting sl's own
// derived characteristics (no recursion through the layer it changes).
func (s *State) eachOverrideStatic(k *prep, sl Slot, f func(o *kop.Override)) {
	s.eachOverrideStaticTs(k, sl, func(o *kop.Override, _ uint16) { f(o) })
}

// eachOverrideStaticTs is eachOverrideStatic with each static's layer
// timestamp (its source permanent's).
func (s *State) eachOverrideStaticTs(k *prep, sl Slot, f func(o *kop.Override, ts uint16)) {
	s.eachOverrideStaticBelow(k, sl, ^kop.TypeMask(0), f)
}

// eachOverrideStaticBelow is eachOverrideStaticTs with the filter's type
// clause checked against below, sl's types before this layer's override
// statics: a static applies only while the host still has a type its
// filter names (a host that stopped being an artifact keeps its own
// types). ^kop.TypeMask(0) accepts every static (eachOverrideStaticTs's
// callers do not need the clause).
func (s *State) eachOverrideStaticBelow(k *prep, sl Slot, below kop.TypeMask, f func(o *kop.Override, ts uint16)) {
	if !k.gen.anyOvrStat {
		return
	}
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		if s.perms[src].attached != sl || src == sl || !k.gen.has(s.cardAt(src), gfOvrStat) {
			continue
		}
		sp := &s.perms[src]
		lost := sp.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOverride && a.Static.Override > 0 {
				if lost && !keptOnLoss(k.t, a) {
					continue // CR 613.6: only an override with a part before layer 6 outlives the loss
				}
				if ft := k.t.Filters[a.Static.Filter].Types; ft != 0 && below&ft == 0 {
					continue // the filter's type clause (basePTGen checks the same one)
				}
				f(&k.t.Overrides[a.Static.Override-1], s.layerTs(src))
			}
		}
	}
}

// eachOverride calls f for sl's own override records then the override
// statics on it, in application order.
func (s *State) eachOverride(k *prep, sl Slot, f func(o *kop.Override, src CardID)) {
	s.eachOvRec(sl, func(r ovRec) {
		f(&k.t.Overrides[r.idx-1], r.src)
	})
	s.eachOverrideStatic(k, sl, func(o *kop.Override) { f(o, NoCard) })
}

const NoCard = kop.NoCard

// typesGen is sl's types.
func (s *State) typesGen(k *prep, sl Slot) kop.TypeMask {
	if k.gen.fxNeed != 0 && s.walkSlot(k, sl) {
		if k.gen.tyPost {
			return s.tyPostTy(k, sl, s.walkTypes(k, sl)) // bestow, impending (kw_impending.go)
		}
		return s.walkTypes(k, sl)
	}
	p := &s.perms[sl]
	t := k.cards[s.cardAt(sl)].types
	if k.gen.cpxTy && s.fxM.has(sl) {
		t |= s.copyExcTypes(k, sl) // layer 1: a copy's added types
		if k.gen.cpxNoLeg && s.copyExcNoLegendary(k, sl) {
			t &^= kop.TLegendary // NonLegendary$ (copy_nonlegendary.go)
		}
	}
	if p.ovr.flags&(ovTypes|ovTypesEOT) != 0 {
		t = p.ovr.types
	}
	apply := func(o *kop.Override) {
		const keep = kop.TLegendary | kop.TBasic | kop.TToken
		if o.HasTypes {
			t = t&(keep|o.KeepTypes) | o.SetTypes
		}
		t |= o.AddTypes
	}
	s.eachOvRec(sl, func(r ovRec) { apply(&k.t.Overrides[r.idx-1]) })
	// The override statics' filter clause is checked against the records'
	// types (the layer below them), never against the result: checking it
	// against t would let a static's own AddType re-admit it.
	s.eachOverrideStaticBelow(k, sl, t, func(o *kop.Override, _ uint16) { apply(o) })
	if k.gen.tyPost {
		t = s.tyPostTy(k, sl, t) // bestow, impending (kw_impending.go)
	}
	return t
}

// colorsGen is sl's colours.
func (s *State) colorsGen(k *prep, sl Slot) kop.ColorMask {
	if k.gen.fxNeed != 0 && s.walkSlot(k, sl) {
		return s.walkColors(k, sl)
	}
	c := k.t.Cards[s.cardAt(sl)].Colors
	if s.fxM.has(sl) && k.gen.cpxCol {
		c = s.copyExcCol(k, sl, c) // layer 1: a copy's colour exceptions
	}
	s.eachOverride(k, sl, func(o *kop.Override, _ CardID) {
		if o.HasColors {
			c = o.SetColors
		}
		if o.AddColors != 0 {
			c = animAddColors(c, o) // static_animategrants.go
		}
	})
	return c
}

// subOutside is a chosen creature type no card of the matchup has
// (TypeVocab 255): only a permanent with all creature types has it
// (Soulstone Sanctuary animated, counted by Banner of Kinship).
const subOutside uint8 = 254

// A chosen outside creature type is perm.chosen = len(Subtypes) + its rank
// among the vocabulary's outside entries, modulo the free codes below 254,
// so two sources' outside choices compare: different codes are different
// types; equal codes are the same type unless the code is aliased (the
// vocabulary has more outside entries than codes), which stays loud
// (act_choosetype.go outsideAliased).

// outsideSub reports whether chosen/subtype code i is an outside type.
func outsideSub(k *prep, i uint8) bool { return i >= k.nsub && i < 255 }

// outsideCount is the number of outside entries in the type vocabulary.
func outsideCount(k *prep) int {
	n := 0
	for _, si := range k.t.TypeVocab {
		if si == 255 {
			n++
		}
	}
	return n
}

// outsideAliased reports whether outside code ch names more than one type.
func outsideAliased(k *prep, ch uint8) bool {
	base := len(k.t.Subtypes)
	return ch == subOutside || int(ch)-base+int(subOutside)-base < outsideCount(k)
}

// typeVocabSub is the perm.chosen value of creature-type option v.
func typeVocabSub(k *prep, v int) uint8 {
	if si := k.t.TypeVocab[v]; si != 255 {
		return si
	}
	rank := 0
	for _, si := range k.t.TypeVocab[:v] {
		if si == 255 {
			rank++
		}
	}
	base := len(k.t.Subtypes)
	return uint8(base + rank%(int(subOutside)-base))
}

// hasSubGen reports whether sl has matchup subtype i (subOutside: a
// creature type outside the matchup's subtypes).
func (s *State) hasSubGen(k *prep, sl Slot, i uint8) bool {
	if k.gen != nil && k.gen.anyImpending && (outsideSub(k, i) || k.gen.creatSub.Has(int(i))) && s.impendDormant(k, sl) {
		return false // CR 205.1a: gone with the creature type
	}
	if k.gen != nil && k.gen.fxNeed != 0 && s.walkSlot(k, sl) {
		return s.walkHasSub(k, sl, i)
	}
	if outsideSub(k, i) {
		// a printed changeling has every creature type (CR 702.73a; gorge
		// sets AllCreatureTypes), until a subtype-setting effect
		all := k.t.Cards[s.cardAt(sl)].AllCreatureTypes || s.fxM.has(sl) && k.gen.cpxSub && s.copyExcAll(k, sl)
		s.eachOverride(k, sl, func(o *kop.Override, _ CardID) {
			if o.HasSub || o.RemoveCreatureSub {
				all = false
			}
			if ovrAllCreat(o) {
				all = true
			}
		})
		return all
	}
	if i == 255 { // none chosen; an outside code was handled above
		return false
	}
	// a copy's added subtype is a copiable value (layer 1): a later
	// subtype-setting effect replaces it (Abyssal Harvester's Nightmare
	// copy of Mild-Mannered Librarian become a Werewolf)
	m := k.t.Cards[s.cardAt(sl)].Subtypes
	all := false
	if s.fxM.has(sl) && k.gen.cpxSub {
		m = m.Or(s.copyExcSubs(k, sl))
		all = s.copyExcAll(k, sl) // a copy's changeling: every creature type, until a later set
	}
	s.eachOverride(k, sl, func(o *kop.Override, _ CardID) {
		if o.HasSub {
			m = m.And(o.KeepSub).Or(o.SetSub)
			all = false
		}
		if o.RemoveCreatureSub && !DebugRmCreatSubKeep {
			m, all = m.AndNot(k.gen.creatSub), false
		}
		m = m.Or(o.AddSub)
		if ovrAllCreat(o) {
			all = true
		}
	})
	if all && (k.gen == nil || k.gen.creatSub.Has(int(i))) || m.Has(int(i)) {
		return true
	}
	// static subtype grants (Skyknight Squire)
	found := false
	s.eachStaticC(k, sclsSub, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || !a.Static.AddSub.Has(int(i)) && !(k.gen.anyAddChosen && s.chosenGrants(k, a, st.sl, i)) {
			return true
		}
		if keep, _ := s.selfGrantKept(k, sl, st.sl); !keep.Has(int(i)) {
			return true // a newer setter on sl overwrote its own grant (static_selfgrant.go)
		}
		if s.statCond(k, st) && s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerSub) {
			found = true
			return false
		}
		return true
	})
	return found
}

// subsGen is hasSubGen reduced to one pass per slot, for ProjectChars: it
// asks every matchup subtype of every battlefield permanent (and
// subOutside), and hasSubGen recomputes the same per-slot work per question.
// The per-slot reduction is i-independent -- the layer walk, the copy
// exceptions, the override pass and the static scan -- only the final mask
// tests vary with i. The contract is hasSubGen's: for every i in
// 0..len(Subtypes)-1 with i < MaxSubtypes, subs.Has(i) is
// s.hasSubGen(k, sl, i), and all is s.hasSubGen(k, sl, subOutside) (the
// outside branch's all creature types; the mask's own changeling arm, the
// non-outside one, is folded into the mask as the creature subtypes it
// covers). It answers all the questions hasSubGen can: only ProjectChars
// calls it (hasSubGen stays the single-subtype query for its other callers
// -- delegating it would slow every single-i query). k.gen must be non-nil.
func (s *State) subsGen(k *prep, sl Slot) (subs kop.SubMask, all bool) {
	dormant := k.gen.anyImpending && s.impendDormant(k, sl)
	if k.gen.fxNeed != 0 && s.walkSlot(k, sl) {
		c := s.chars(k, sl, upTo4) // one walk, not one per subtype
		for i := range k.t.Subtypes {
			if i < kop.MaxSubtypes && hasSubCur(k, &c, uint8(i)) {
				subs = subs.With(i)
			}
		}
		all = hasSubCur(k, &c, subOutside)
	} else {
		cd := &k.t.Cards[s.cardAt(sl)]
		m := cd.Subtypes
		copyAll := s.fxM.has(sl) && k.gen.cpxSub && s.copyExcAll(k, sl)
		mAll := copyAll                          // the matchup subtypes' changeling: a copy's (a printed one is outside-only)
		allOut := cd.AllCreatureTypes || copyAll // subOutside's: printed or copy changeling
		if s.fxM.has(sl) && k.gen.cpxSub {
			// a copy's added subtype is a copiable value (layer 1): a later
			// subtype-setting effect replaces it
			m = m.Or(s.copyExcSubs(k, sl))
		}
		s.eachOverride(k, sl, func(o *kop.Override, _ CardID) {
			if o.HasSub {
				m = m.And(o.KeepSub).Or(o.SetSub)
				mAll, allOut = false, false
			}
			if o.RemoveCreatureSub {
				allOut = false // the outside branch clears it on the record, mutation switch or not
				if !DebugRmCreatSubKeep {
					m = m.AndNot(k.gen.creatSub)
					mAll = false
				}
			}
			m = m.Or(o.AddSub)
			if ovrAllCreat(o) {
				mAll, allOut = true, true
			}
		})
		subs = m
		if mAll {
			subs = subs.Or(k.gen.creatSub) // every creature type until a setter (CR 702.73a)
		}
		// static subtype grants (Skyknight Squire): one scan, not one per
		// subtype. Each grant's mask (AddSub plus its own chosen type), keep
		// mask and condition are i-independent, so its contribution unions
		// in once: for i, a grant holds exactly when i is in its mask, i is
		// kept and its condition held -- what hasSubGen's per-subtype scan
		// tests.
		s.eachStaticC(k, sclsSub, func(st statSrc) bool {
			a := st.a
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous {
				return true
			}
			add := a.Static.AddSub
			if k.gen.anyAddChosen {
				add = s.chosenGrantMask(k, a, st.sl, add)
			}
			if add == (kop.SubMask{}) {
				return true
			}
			keep, _ := s.selfGrantKept(k, sl, st.sl)
			if !s.statCond(k, st) || !s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerSub) {
				return true
			}
			subs = subs.Or(add.And(keep))
			return true
		})
		all = allOut
	}
	if dormant {
		// CR 205.1a: gone with the creature type -- dormant kills the
		// creature subtypes and subOutside (all), never a non-creature
		// subtype, so it applies on the final union, not by skipping the
		// grants
		subs = subs.AndNot(k.gen.creatSub)
		all = false
	}
	return subs, all
}

// basePTGen is sl's base power/toughness before counters and pumps.
func (s *State) basePTGen(k *prep, sl Slot) (int, int) {
	if k.gen.fxNeed != 0 && s.walkSlot(k, sl) {
		return s.walkBasePT(k, sl)
	}
	p := &s.perms[sl]
	cid := s.cardAt(sl)
	pc := &k.cards[cid]
	bp, bt := int(pc.power), int(pc.tough)
	if k.gen.has(cid, gfCDAPT) && !s.genLost(k, sl) {
		cd := &k.t.Cards[cid]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCDAPT {
				ctx := evalCtx{src: p.obj, srcSl: sl, ctrl: p.ctrl}
				bp, bt = s.evalV(k, a.Static.VA, ctx), s.evalV(k, a.Static.VB, ctx)
			}
		}
	}
	if s.fxM.has(sl) && k.gen.cpxPT {
		// layer 1: a copy's set P/T (CR 707.9b, 707.9d; copy_exceptions.go)
		if cp, ct, ok := s.copyExcPT(k, sl); ok {
			bp, bt = cp, ct
		}
	}
	if p.ovr.flags&(ovSetPT|ovSetPTEOT) != 0 {
		bp, bt = int(p.ovr.setP), int(p.ovr.setT)
	}
	if s.fxM.has(sl) {
		// eachOverridePT inlined: an index loop keeps basePTGen inlinable
		// (no closure in a walk)
		n := 0
		for i := 0; i < int(s.g.nfx) && n < maxOvr; i++ {
			r := &s.g.fx[i]
			if r.sl != sl || r.kind != fxOverride {
				continue
			}
			n++
			o := &k.t.Overrides[r.r.idx-1]
			if !o.HasPT {
				continue
			}
			rp, rt := r.r.p, r.r.t
			if o.LivePT {
				if s.livePTLost(k, sl, r.r.ts) {
					continue
				}
				rp, rt = s.livePT(k, sl, o, rp) // static_animategrants.go
			} else if o.WhileHost {
				rp, rt = int16(o.SetP), int16(o.SetT) // r.r.p/t hold the host
			}
			if !o.KeepP {
				bp = int(rp)
			}
			if !o.KeepT {
				bt = int(rt)
			}
		}
	}
	if k.gen.anyOvrStat {
		for m := s.statM; m.any(); m = m.rest() {
			src := m.first()
			if s.perms[src].attached != sl || src == sl || !k.gen.has(s.cardAt(src), gfOvrStat) {
				continue
			}
			lost := s.perms[src].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0
			cd := &k.t.Cards[s.cardAt(src)]
			for i := range cd.Abilities {
				a := &cd.Abilities[i]
				if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatOverride || a.Static.Override <= 0 || lost && !keptOnLoss(k.t, a) {
					continue
				}
				o := &k.t.Overrides[a.Static.Override-1]
				if !o.HasPT {
					continue
				}
				if ft := k.t.Filters[a.Static.Filter].Types; ft != 0 && s.typesGen(k, sl)&ft == 0 {
					continue // the filter's type clause: a host that stopped being a creature keeps its own P/T
				}
				rp, rt := int16(o.SetP), int16(o.SetT)
				if o.LivePT {
					rp, rt = s.livePTSrc(k, src, o) // static_animategrants.go
				}
				if !o.KeepP {
					bp = int(rp)
				}
				if !o.KeepT {
					bt = int(rt)
				}
			}
		}
	}
	return bp, bt
}

// ptGen is sl's power and toughness.
func (s *State) ptGen(k *prep, sl Slot) (int, int) {
	p := &s.perms[sl]
	bp, bt := s.basePTGen(k, sl)
	c := p.ctrLow(int(kop.CtrP1P1)) - p.ctrLow(int(kop.CtrM1M1))
	ep, et := int(p.eotP), int(p.eotT)
	if s.nptx != 0 {
		ep, et = s.eotPT(sl)
	}
	bp += c + ep
	bt += c + et
	if k.ctrMana {
		bt -= p.ctrN(s, int(kop.CtrM0M1)) // -0/-1 counters (mana_countercost.go)
	}
	if k.gen.anyPTDelta && s.fxM.has(sl) {
		dp, dt := s.ptDeltaRecs(k, sl) // layer 7c override pumps (static_ptdelta.go)
		bp += dp
		bt += dt
	}
	if k.gen.anyHone {
		bp += s.honePower(k, sl) // CR 122.1j hone counters on attached permanents (ctr_hone.go)
	}
	if s.statM.any() || s.g.nemb[0]+s.g.nemb[1] != 0 {
		s.eachStaticC(k, sclsPT, func(st statSrc) bool {
			a := st.a
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous {
				return true
			}
			sa := &a.Static
			if sa.StFlags&kop.StDynamic == 0 && sa.A == 0 && sa.B == 0 {
				return true
			}
			if !s.statCond(k, st) || !s.matchPermGen(k, sa.Filter, sl, st.ctrl, st.sl, layerPT) {
				return true
			}
			if sa.StFlags&kop.StDynamic != 0 {
				ctx := s.ctxStatic(st)
				bp += s.evalV(k, sa.VA, ctx)
				bt += s.evalV(k, sa.VB, ctx)
			} else {
				bp += int(sa.A)
				bt += int(sa.B)
			}
			return true
		})
	}
	return bp, bt
}

// kwGen is sl's keywords.
func (s *State) kwGen(k *prep, sl Slot) kop.KwMask {
	if s.l6On(k, sl) {
		return s.kw6(k, sl) // CR 613 layer 6 walk (static_layer6.go)
	}
	p := &s.perms[sl]
	var m kop.KwMask
	lts, lost := s.lossAt(k, sl)
	if !lost {
		m = k.cards[s.cardAt(sl)].kw
	}
	m |= p.eotKw | p.ovr.kw
	if !lost && s.nkwx != 0 {
		m |= s.supKw(sl) // grants an ended loss had suppressed
	}
	s.eachOverrideTs(k, sl, func(o *kop.Override, ts uint16) {
		if !lost || ts >= lts {
			m |= o.AddKw
		}
	})
	// CR 122.1b + 613.1c: the counter grant is a BASE keyword (gorge
	// BaseKeywords, engine/rules/chars/derive.go), so an active
	// loses-all-abilities effect clears it too (Abigale's Animate then
	// keyword counters); a later layer-6 grant still re-adds on top.
	if !lost {
		m |= ctrKw(s, p)
		if k.gen.anySuspect {
			m |= s.suspectKw(k, sl) // CR 701.60a: a base keyword, as the cloak's ward (static_layer6.go)
		}
	}
	if s.statM.any() || s.g.nemb[0]+s.g.nemb[1] != 0 {
		s.eachStaticC(k, sclsKw, func(st statSrc) bool {
			a := st.a
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.Kw == 0 || m&a.Static.Kw == a.Static.Kw {
				return true
			}
			if lost && s.grantTs(st) < lts {
				return true
			}
			if s.statCond(k, st) && s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
				m |= a.Static.Kw
			}
			return true
		})
	}
	return m
}

// eachOverrideTs is eachOverride with each effect's layer timestamp (a
// record's own, an attached override static's source's) in place of the
// granting card.
func (s *State) eachOverrideTs(k *prep, sl Slot, f func(o *kop.Override, ts uint16)) {
	s.eachOvRec(sl, func(r ovRec) {
		f(&k.t.Overrides[r.idx-1], r.ts)
	})
	s.eachOverrideStaticTs(k, sl, f)
}

// grantedAbilities calls f(card, ability index) for every ability the
// permanent at sl gains from an override record or a static grant.
func (s *State) grantedAbilities(k *prep, sl Slot, f func(c CardID, ab uint8)) {
	s.grantedAbilitiesN(k, sl, func(c CardID, ab uint8, _ int) { f(c, ab) })
}

// grantedAbilitiesN is grantedAbilities with how many times each grant
// applies: an override record merges identical applications (two Fake
// Your Own Deaths on one creature), and each is its own instance of the
// granted ability (two dies triggers, as gorge lists them).
func (s *State) grantedAbilitiesN(k *prep, sl Slot, f func(c CardID, ab uint8, n int)) {
	s.grantedAbilitiesOwn(k, sl, nil, f)
}

// grantedAbilitiesOwn is grantedAbilitiesN with own, when not nil, called
// once at sl's own timestamp among the statics' grants: the triggers sl's
// own static grants it (a Station threshold's), listed as gorge lists a
// static's grant (slotTriggers).
func (s *State) grantedAbilitiesOwn(k *prep, sl Slot, own func(), f func(c CardID, ab uint8, n int)) {
	if !k.gen.anyGrant {
		if own != nil {
			own()
		}
		return
	}
	ots := uint32(s.layerTs(sl))
	// a grant older than a "loses all abilities" effect on sl is removed
	// with the rest (gorge grantLostTo); the loss is read only once a
	// grant is found (this walk runs per trigger listener)
	var lts uint16
	lost, read := false, false
	// With a static in play, gorge's AddTrigger$ grants of statics are
	// effects of layer 0 and list ahead of the effects registered at layer 6
	// (an Animate grant, a Pump's), however their timestamps compare; a
	// granted keyword's trigger (Firebending) is a layer-6 effect and keeps
	// its place by timestamp. wm is the timestamp the walk has reached: the
	// keyword records and the attached statics stamped before the next
	// global grant are listed first (grantsBetween), then that grant, so
	// the statics come out in one timestamp order, then the plain records.
	wm := uint32(0)
	kinds := uint8(grantAny)
	if k.gen.globGrant || s.grantM.any() {
		kinds = grantKw
	}
	if k.gen.globGrant {
		lts, lost = s.lossAt(k, sl)
		read = true
		g := func(c CardID, ab uint8, n int, ts uint16) {
			if own != nil && uint32(ts) > ots {
				lts, lost, read = s.grantsBetween(k, sl, wm, ots, kinds, lts, lost, read, f)
				wm = ots
				own()
				own = nil
			}
			lts, lost, read = s.grantsBetween(k, sl, wm, uint32(ts), kinds, lts, lost, read, f)
			wm = uint32(ts)
			f(c, ab, n)
		}
		s.globalGrants(k, sl, lost, lts, g)
		s.globalGrantsGen(k, sl, lost, lts, g)
	}
	if own != nil {
		if ots >= wm {
			lts, lost, read = s.grantsBetween(k, sl, wm, ots, kinds, lts, lost, read, f)
			wm = ots
		}
		own()
	}
	lts, lost, read = s.grantsBetween(k, sl, wm, 1<<16, kinds, lts, lost, read, f)
	if kinds == grantKw && s.fxM.has(sl) {
		// the triggers an effect grants come after those a static grants
		s.ovGrantsBetween(k, sl, 0, 1<<16, lost, lts, read, grantPlain, f)
	}
	if k.gen.anyContGrant {
		// a Continuous grant of a mana ability from an unattached source
		// (Springleaf Parade's tokens; mana_grantedoffers.go)
		if !read {
			lts, lost = s.lossAt(k, sl)
		}
		s.contGrants(k, sl, lost, lts, f)
	}
}

// grantsBetween lists, in layer-6 timestamp order, the grants of the override
// records on sl and of the statics attached to it stamped in [lo, hi). Gorge
// lists granted abilities in timestamp order across every source (an
// Equipment's timestamp is restamped when it is attached, CR 613.7e), so
// two Equipment on one creature queue their triggers oldest attachment first
// and an Animate grant stamped between them falls between them; the
// controller's trigger-order choice indexes that queue. The records between
// two attached statics keep pool order (ovGrantsBetween). It returns the
// loss read (lts, lost, read) for the next call.
func (s *State) grantsBetween(k *prep, sl Slot, lo, hi uint32, kinds uint8, lts uint16, lost, read bool, f func(c CardID, ab uint8, n int)) (uint16, bool, bool) {
	cur := lo
	for m := s.grantM; m.any(); {
		src, found := NoSlot, false
		var best uint16
		for r := m; r.any(); r = r.rest() {
			c := r.first()
			if s.perms[c].attached != sl {
				continue
			}
			if ts := s.layerTs(c); uint32(ts) >= cur && uint32(ts) < hi && (!found || ts < best) {
				src, best, found = c, ts, true
			}
		}
		if !found {
			break
		}
		m = m.without(src)
		if s.genLost(k, src) {
			continue
		}
		if !read {
			lts, lost = s.lossAt(k, sl)
			read = true
		}
		if lost && s.layerTs(src) < lts {
			continue
		}
		if s.fxM.has(sl) {
			s.ovGrantsBetween(k, sl, cur, uint32(best), lost, lts, true, kinds, f)
		}
		cur = uint32(best)
		cid := s.cardAt(src)
		cd := &k.t.Cards[cid]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic {
				continue
			}
			switch a.Static.Kind {
			case kop.StatOverride:
				for _, g := range k.t.Overrides[a.Static.Override-1].Grant {
					if g != 0 {
						prev := s.g.grantSrc
						s.g.grantSrc = uint8(s.perms[src].obj) + 1
						f(cid, g-1, 1)
						s.g.grantSrc = prev
					}
				}
			case kop.StatContinuous:
				st := statSrc{a: a, sl: src, ctrl: s.perms[src].ctrl, obj: s.perms[src].obj}
				if a.Static.Grant != 0 && s.statCond(k, st) && s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, src, layerKw) {
					prev := s.g.grantSrc
					s.g.grantSrc = uint8(s.perms[src].obj) + 1
					f(cid, a.Static.Grant-1, 1)
					s.g.grantSrc = prev
				}
			}
		}
	}
	if s.fxM.has(sl) {
		s.ovGrantsBetween(k, sl, cur, hi, lost, lts, read, kinds, f)
	}
	return lts, lost, read
}

// The kinds of override-record grant ovGrantsBetween lists: every one, a
// granted keyword's trigger (kop.Ability.GrantedKw), or any other.
const (
	grantAny = iota
	grantKw
	grantPlain
)

// ovGrantsBetween calls f for the abilities the override records on sl
// stamped in [lo, hi) grant, in pool order (the pool walked in place:
// eachOvRec's closure would put the caller's state in memory). known says
// lost and lts are already read; otherwise the loss is read at the first
// record.
func (s *State) ovGrantsBetween(k *prep, sl Slot, lo, hi uint32, lost bool, lts uint16, known bool, kinds uint8, f func(c CardID, ab uint8, n int)) {
	for i, n := 0, 0; i < int(s.g.nfx) && n < maxOvr; i++ {
		r := s.g.fx[i].r
		if s.g.fx[i].sl != sl || s.g.fx[i].kind != fxOverride {
			continue
		}
		n++
		if uint32(r.ts) < lo || uint32(r.ts) >= hi {
			continue
		}
		if !known {
			lts, lost = s.lossAt(k, sl)
			known = true
		}
		if lost && r.ts < lts {
			continue
		}
		o := &k.t.Overrides[r.idx-1]
		for _, g := range o.Grant {
			if g == 0 {
				continue
			}
			if kinds != grantAny && (k.t.Cards[r.src].Abilities[g-1].GrantedKw) != (kinds == grantKw) {
				continue
			}
			f(r.src, g-1, r.reps())
		}
	}
}

// grantedHost returns the permanent granting sl the ability (c, ab) (the
// Equipment attached to it), NoSlot if none: of two copies, the older whose grant
// is not itself older than sl's loss of all abilities (two Sugar Coats).
func (s *State) grantedHost(k *prep, sl Slot, c CardID) Slot {
	host := NoSlot
	var lts uint16
	lost, read := false, false
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		if s.perms[src].attached != sl || s.cardAt(src) != c {
			continue
		}
		if !read {
			lts, lost = s.lossAt(k, sl)
			read = true
		}
		if ts := s.layerTs(src); !(lost && ts < lts) && (host == NoSlot || ts < s.layerTs(host)) {
			host = src
		}
	}
	return host
}

// selfGrantHost returns sl when a live override record on sl made by sl's
// own card grants it ability (c, ab) and sl has not lost its abilities
// (Domesticated Watercourse's "becomes an Equipment with equip {2}"); else
// NoSlot. A loss in the same record (The Irencrag's Everflame) hosts nothing:
// gorge offers no equip there.
//
// On a copy, an as-enters copy's AddAbilities$ record (Mercurial Pretender,
// copy_asenters.go) hosts too, whichever card made it: the Pretender's own
// copy, and a copy of that copy (CR 707.3), whose ability grantCardAt then
// reads from the card that made the record.
func (s *State) selfGrantHost(k *prep, sl Slot, c CardID, ab uint8) Slot {
	if !s.fxM.has(sl) || s.genLost(k, sl) {
		return NoSlot
	}
	if (s.cardAt(sl) == c || s.perms[sl].card == c) && s.ownGrants(k, sl, c, ab) {
		return sl
	}
	if own, ok := s.copyGrantCard(k, sl, ab); ok && own == c {
		return sl
	}
	return NoSlot
}

// copyGrantCard is the card of the first as-enters copy grant record on sl
// that grants ability ab.
func (s *State) copyGrantCard(k *prep, sl Slot, ab uint8) (CardID, bool) {
	for i := 0; i < int(s.g.nfx); i++ {
		r := &s.g.fx[i]
		if r.sl != sl || r.kind != fxOverride || r.r.idx == 0 || !r.r.copyGrant() {
			continue
		}
		for _, g := range k.t.Overrides[r.r.idx-1].Grant {
			if g == ab+1 {
				return r.r.src, true
			}
		}
	}
	return 0, false
}

// copyGrantIs reports whether ability (c, ab) of sl is an as-enters copy's
// grant: a copiable value (CR 707.9b), so gorge offers it as a printed
// ability, priced off every source the controller can tap, not the pool only.
func (s *State) copyGrantIs(k *prep, sl Slot, c CardID, ab uint8) bool {
	own, ok := s.copyGrantCard(k, sl, ab)
	return ok && own == c
}

// ownGrants reports whether a live override record on sl made by card c
// grants sl c's ability ab.
func (s *State) ownGrants(k *prep, sl Slot, c CardID, ab uint8) bool {
	if !s.fxM.has(sl) {
		return false
	}
	for i := 0; i < int(s.g.nfx); i++ {
		r := &s.g.fx[i]
		if r.sl != sl || r.kind != fxOverride || r.r.idx == 0 || r.r.src != c {
			continue
		}
		for _, g := range k.t.Overrides[r.r.idx-1].Grant {
			if g == ab+1 {
				return true
			}
		}
	}
	return false
}

// grantCardAt is the card whose ability ab a granted activation (mode
// 0x80|ab) hosted by the battlefield permanent at sl names: sl's current
// card, or, on a copy, the card of the as-enters grant record that grants sl
// that ability when no record of the current card does (selfGrantHost).
func (s *State) grantCardAt(k *prep, sl Slot, ab uint8) CardID {
	c := s.cardAt(sl)
	if !s.ownGrants(k, sl, c, ab) {
		// also on a permanent whose printed card is the one it shows: a
		// Grizzly Bears turned into a copy of a Pretender copy by a permanent
		// copy effect holds the Pretender's grant (copyGrantsAdd)
		if own, ok := s.copyGrantCard(k, sl, ab); ok {
			return own
		}
	}
	return c
}

// applyOverride records override idx (from card src) on sl.
func (s *State) applyOverride(k *prep, sl Slot, idx int, src CardID, eot bool) {
	var fl uint8
	if eot {
		fl = ovEOT
	}
	s.applyOverrideFl(k, sl, idx, src, fl)
}

// applyOverrideFl is applyOverride with the record's duration bits.
func (s *State) applyOverrideFl(k *prep, sl Slot, idx int, src CardID, fl uint8) {
	p, t := s.overridePT(k, idx)
	if k.t.Overrides[idx].LivePT {
		p, t = animLiveCtrl(s) // static_animategrants.go
	}
	s.applyOverridePT(k, sl, idx, src, fl, p, t)
}

// applyOverridePT applies override idx with its P/T already evaluated.
func (s *State) applyOverridePT(k *prep, sl Slot, idx int, src CardID, fl uint8, p, t int16) {
	r := ovRec{idx: uint8(idx + 1), src: src, ts: s.tick(), fl: fl, p: p, t: t}
	// merge: a re-application accumulates (the reps count) in place. A
	// record that OVERWRITES colours must not merge: gorge's re-applied
	// animate grant is a new effect whose layer-5 colour REPLACES, so the
	// newest application wins (CR 613.5; Mondo Gecko re-activated in a new
	// colour). An ADDITIVE colour record has the
	// same ordering against any setter applied in between (add blue, become
	// red, add blue again: red and blue). HasPT records already take the
	// newest position (the move-to-end below). A types or subtypes SETTER
	// is ordered the same way (CR 613.7, a new effect gets a new
	// timestamp): Opal Acrolith turns into an enchantment, is animated,
	// then turns back; a merged second turn-back sat before the animate and
	// the object stayed a creature.
	ov := &k.t.Overrides[idx]
	merge := !ov.HasPT && !ov.HasColors && ov.AddColors == 0 && !ov.HasTypes && !ov.HasSub
	if merge && k.gen.anyOvrStat && s.ovStaticOn(k, sl) {
		// An override static on the host (an Aura that removes card types)
		// sits between the older application and this one in timestamp
		// order, so the newer application takes the newest position too
		// (Crew again on a Vehicle under Eaten by Piranhas: artifact again).
		merge = false
	}
	// A keyword grant or removal is a layer-6 effect ordered by its timestamp
	// (CR 613.1f, 613.7), like a granted ability: it merges with the newest record only, or
	// a repeat of The Fire Nation Drill's removal resolved after Enter the Avatar State would
	// sit before it and leave the hexproof the Avatar gave.
	if !s.addOvRec(sl, r, merge, ov.Grant[0] != 0 || ov.AddKw != 0 || ov.RemoveKw != 0 || ov.LoseAbilities) {
		s.stall(StallTables)
		return
	}
	if k.t.Overrides[idx].LoseAbilities {
		s.loseAllOnset(k, NoSlot, sl)
	}
	s.subscribeGrants(k, sl)
	s.refreshTy(k)
}

// subscribeGrants subscribes sl to the events of its granted triggers.
func (s *State) subscribeGrants(k *prep, sl Slot) {
	s.grantedAbilities(k, sl, func(c CardID, ab uint8) {
		a := &k.t.Cards[c].Abilities[ab]
		if a.Kind == kop.AbTriggered && a.Trig.Event < nSubEvents {
			s.subs[a.Trig.Event].set(sl)
		}
	})
}

// grantedKwGen calls add with the keywords sl gains from each override
// record and each Continuous static whose conditions hold, one call per
// effect (dirac/feat's granted list).
func (s *State) grantedKwGen(k *prep, sl Slot, add func(kop.KwMask)) {
	if s.l6On(k, sl) {
		s.grantedKw6(k, sl, add)
		return
	}
	lts, lost := s.lossAt(k, sl)
	s.eachOvRec(sl, func(r ovRec) {
		if !lost || r.ts >= lts {
			if o := &k.t.Overrides[r.idx-1]; o.AddKw != 0 {
				for j := 0; j < r.reps(); j++ {
					add(o.AddKw) // once per application
				}
			}
		}
	})
	s.eachOverrideStaticTs(k, sl, func(o *kop.Override, ts uint16) {
		if o.AddKw != 0 && (!lost || ts >= lts) {
			add(o.AddKw)
		}
	})
	if s.statM.any() || s.g.nemb[0]+s.g.nemb[1] != 0 {
		s.eachStaticC(k, sclsKw, func(st statSrc) bool {
			a := st.a
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.Kw != 0 && (!lost || s.grantTs(st) >= lts) &&
				s.statCond(k, st) && s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
				add(a.Static.Kw)
			}
			return true
		})
	}
}

// FeatNameCard is the card that names battlefield object o when an
// override renamed it (Witness Protection's Legitimate Businessperson,
// kop.Override.NameCard); false when its own card names it.
func (s *State) FeatNameCard(o Obj) (CardID, bool) {
	ob := &s.objs[o]
	k := s.k()
	if k.gen == nil || ob.zone != kop.ZoneBattlefield {
		return 0, false
	}
	if k.gen.fxNeed&(fxNeedL6|fxNeedGlobal) != 0 || s.perms[ob.slot].ovr.walk&walkOn != 0 {
		return s.walkName(k, ob.slot) // the layer walk's layer 3 (static_layer6.go)
	}
	nc := uint16(0)
	s.eachOverride(k, ob.slot, func(ov *kop.Override, _ CardID) {
		if ov.NameCard != 0 {
			nc = ov.NameCard
		}
	})
	if nc == 0 {
		return 0, false
	}
	return CardID(nc - 1), true
}
