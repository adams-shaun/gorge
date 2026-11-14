package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Statics on a generated table beyond the P/T and keyword sums
// (chars_gen.go): rule statics with conditions, cost reductions that work
// from the card itself, play-from-zone permissions, and the "can't lose"
// replacements.

// staticFlagGen reports whether a static of kind sk applies to slot sl.
func (s *State) staticFlagGen(k *prep, sk kop.StaticKind, sl Slot) bool {
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind == kop.AbStatic && a.Static.Kind == sk && s.statCond(k, st) && s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerFull) {
			hit = true
			return false
		}
		return true
	})
	if !hit && k.gen.anyOvGateStat && s.fxM.has(sl) {
		// a static an Animate staticAbilities$ grant record on sl switches on
		// (Streetwise Negotiator's Backup rider; static_ovgate.go)
		s.grantedStaticFlag(k, sk, sl, &hit)
	}
	return hit
}

// blockForbiddenGen applies CantBlock / CantBlockBy statics (with their
// conditions) and protection from a colour (CR 702.16e: can't be blocked by
// creatures of that colour).
func (s *State) blockForbiddenGen(k *prep, b, a Slot) bool {
	if k.gen.anyColGrant {
		if _, nb, _ := s.cgOf(a); nb&s.colorsGen(k, b) != 0 {
			return true // a chosen-colour CantBlockBy (static_colourgrant.go)
		}
	}
	akw := s.kwGen(k, a)
	if akw&(kop.KwProtBlack|kop.KwProtRed) != 0 {
		c := s.colorsGen(k, b)
		if akw&kop.KwProtBlack != 0 && c&kop.ColB != 0 || akw&kop.KwProtRed != 0 && c&kop.ColR != 0 {
			return true
		}
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		ab := st.a
		if ab.Kind != kop.AbStatic || !s.statCond(k, st) {
			return true
		}
		switch ab.Static.Kind {
		case kop.StatCantBlock:
			hit = s.matchPermGen(k, ab.Static.Filter, b, st.ctrl, st.sl, layerFull)
		case kop.StatCantBlockBy:
			hit = s.matchPermGen(k, ab.Static.Filter, a, st.ctrl, st.sl, layerFull) &&
				(ab.Static.Filter2 == 0 || s.matchPermGen(k, ab.Static.Filter2, b, st.ctrl, st.sl, layerFull)) &&
				(!k.anyPwVsSrc || s.pwVsSrc(k, b, st.sl, ab.Static.PwVsSrc))
		}
		return !hit
	})
	return hit
}

// adjustCostGen applies cost reductions to card c (object o) cast by p:
// ReduceCost statics of p's permanents, and "this spell costs less"
// statics on the card itself (Zone == ZoneCommand), whose amount is
// evaluated with the card as the source. A reduction that depends on the
// spell's target (StTargetsFilter2) applies once a matching target is
// chosen; before targets (the priority walk) when one could be.
func (s *State) adjustCostGen(k *prep, p uint8, c CardID, o Obj, base kop.ManaCost) kop.ManaCost {
	return s.adjustCostGenB(k, p, c, o, o, base, k.bargainRed && s.bargainCharge(k, c))
}

// adjustCostGenB is adjustCostGen for the bargained cast when bargained: the
// StBargained statics apply only then (cost_reducespell.go). priced is
// the card the statics' zone counts skip (evalCtx.uncounted), NoObj for none.
func (s *State) adjustCostGenB(k *prep, p uint8, c CardID, o, priced Obj, base kop.ManaCost, bargained bool) kop.ManaCost {
	g := int(base.Generic)
	if s.g.raiseO|s.g.raiseB != 0 {
		g += s.partitionRaise(o) // a may-play surcharge, before the cost statics (cast_whileexiled.go)
	}
	// the spell being priced on the stack counts its announced X in the mana
	// value a cost static tests (CR 202.3e, 601.2f); off the stack it has none
	mo := o
	if mo == NoObj && priced != NoObj && s.objs[priced].zone == kop.ZoneStack {
		mo = priced
	}
	if k.gen.anyRaise {
		// CR 601.2f: the raises come first, so a reduction takes the raised
		// pips too (gorge CostMods.ApplyTo)
		g += s.raiseCostGen(k, p, c, o, priced, &base.Pips)
	}
	apply := func(a *kop.Ability, ctx evalCtx, emb uint8) {
		ctx.uncounted = priced + 1
		st := &a.Static
		if st.Cond2 != 0 && !s.condEval(k, st.Cond2, ctx) {
			return
		}
		if !s.matchSpellCard(k, st.Filter, c, p, ctx.ctrl, mo, ctx.srcSl) {
			return
		}
		if st.StFlags&kop.StTargetsFilter2 != 0 && (s.g.noTgtRed || !s.costTargetOK(k, st.Filter2, p, c)) {
			return
		}
		if st.StFlags&(kop.StBargained|kop.StReduceColor) != 0 {
			if st.StFlags&kop.StBargained != 0 && (!bargained || DebugReduceSpellFlip) {
				return
			}
			if st.StFlags&kop.StReduceColor != 0 {
				g -= reduceColorTake(&base, st.B)
			}
		}
		if st.StFlags&kop.StEmbNum != 0 {
			// the Effect's frozen SetChosenNumber$:
			// gorge reads state.ContinuousEffect.ChosenNumber, never the
			// live Count$ChosenNumber source
			v := s.embNumAt(emb)
			if DebugEmbNumLive {
				v = s.embNumLive(k, emb) // TestEmbnumMutation only
			}
			if v > 0 {
				g -= int(v)
			}
		} else if st.StFlags&kop.StDynamic != 0 {
			if v := s.evalV(k, st.VA, ctx); v > 0 { // gorge modAmountX: a negative amount is no reduction
				g -= v
			}
		} else {
			g -= int(st.A)
		}
	}
	s.eachStaticC(k, sclsReduce, func(stc statSrc) bool {
		a := stc.a
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatReduceCost && a.Static.Zone != kop.ZoneCommand && (stc.ctrl == p || a.Static.StFlags&kop.StReduceAny != 0) {
			apply(a, s.ctxStatic(stc), stc.emb)
		}
		return true
	})
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatReduceCost && a.Static.Zone == kop.ZoneCommand {
			apply(a, evalCtx{src: mo, srcSl: NoSlot, ctrl: p}, 0) // (mo: the priced spell on the stack reads its announced X, CR 107.3, 601.2f)
		}
	}
	if cd.SpellWaterbend != 0 {
		own := o
		if own == NoObj {
			own = priced
		}
		g += s.waterbendOwnPart(k, p, c, own) // the spell's own waterbend {N} (RaiseCost Cost$ Waterbend<N>)
	}
	if g < 0 {
		if base.X != 0 {
			base.XRed = uint8(min(-g, 255)) // CR 601.2f: the excess pays into {X}
		}
		g = 0
	}
	base.Generic = uint8(g)
	return base
}

// matchSpellCard is matchCardGen for a spell whose cost is being
// determined: its cost is read as it will be on the stack (CR 601.2f), so a
// filter's "on the stack" zone holds for a card still in its hand, exile or
// graveyard (Tolarian Terror's self reduction cast from its owner's
// graveyard through Muldrotha: pair 4094; from another player's stash
// through Tinybones: pairs 8835 9227 9239, gorge reading the caster as
// "you" since f04ea781f).
//
// src is the slot of the static's source permanent (NoSlot for a stack
// spell's own grant): a ChosenType alternative reads that permanent's
// recorded choice (Gathering Stone's ReduceCost Card.ChosenType; gorge
// sourceChosen), a printed changeling having every type.
func (s *State) matchSpellCard(k *prep, f kop.FilterID, c CardID, owner, ctrl uint8, o Obj, src Slot) bool {
	if f == 0 {
		return true
	}
	for f != 0 {
		fl := k.t.Filters[f]
		if s.matchSpellAlt(k, fl, c, owner, ctrl, o, src) {
			return true
		}
		f = fl.Or
	}
	return false
}

// matchSpellAlt is one alternative of matchSpellCard's Or chain (fl is a
// copy: the zone and chosen-type words are rewritten on it).
func (s *State) matchSpellAlt(k *prep, fl kop.Filter, c CardID, owner, ctrl uint8, o Obj, src Slot) bool {
	if fl.Zone == kop.ZoneStack {
		fl.Zone = kop.ZoneNone
	}
	if src != NoSlot && fl.Flags2&kop.F2ChosenType != 0 {
		ch, ok := s.chosenTypeSub(src)
		cd := &k.t.Cards[c]
		has := ok && int(ch) < len(k.t.Subtypes) && cd.Subtypes.Has(int(ch))
		if ok && outsideSub(k, ch) {
			has = cd.AllCreatureTypes
		}
		fl.Flags2 &^= kop.F2ChosenType
		return has && s.matchCardOne(k, &fl, c, owner, ctrl, o)
	}
	return s.matchCardOne(k, &fl, c, owner, ctrl, o)
}

// targetsAnswered reports whether the build's target stages have passed: the
// chosen set (none included) is what a target-dependent price reads then
// (CR 601.2c before 601.2f), not the candidate census that priced the ask.
func (s *State) targetsAnswered() bool {
	switch s.bld().stage {
	case bsDivide, bsEvidence, bsMana, bsCosts, bsPay:
		return true
	}
	return false
}

// costTargetOK: the spell being built targets an object matching f, or (no
// target chosen yet) some object could be such a target: a battlefield
// object, or for a stack-zone f (a Spell head: Out of Air's Spell.Creature)
// a spell on the stack (costTargetMatch, filter_heads.go).
func (s *State) costTargetOK(k *prep, f kop.FilterID, p uint8, c CardID) bool {
	if s.ctl == ctlBuild && (s.bld().e.nt > 0 || s.targetsAnswered()) { // once the ask is answered, none means an "up to" answered with none
		e := &s.bld().e
		for i := 0; i < int(e.nt); i++ {
			if s.costTargetMatch(k, f, p, e.tgt[i]) {
				return true
			}
		}
		x := s.xtOf(e)
		for j := 0; j < int(x.n); j++ {
			if s.costTargetMatch(k, f, p, x.code[j]) {
				return true
			}
		}
		return false
	}
	t := tspec(k, c, spellAb(k, c), 0)
	if t == nil {
		return false
	}
	var buf [MaxOptions]TCode
	pr := s.g.probe
	if pr.kind == 0 {
		s.g.probe = probe{kind: probeSpell, src: NoSlot, card: c, ctrl: p}
	}
	n := s.targetCands(k, t, p, NoSlot, NoObj, &buf)
	s.g.probe = pr
	for i := 0; i < n; i++ {
		if s.costTargetMatch(k, f, p, buf[i]) {
			return true
		}
	}
	return false
}

// landLimit is the number of lands p may play this turn.
func (s *State) landLimit(k *prep, p uint8) uint8 {
	n := 1
	if k.gen == nil {
		return uint8(n)
	}
	s.eachStatic(k, func(st statSrc) bool {
		if st.a.Kind == kop.AbStatic && st.a.Static.Kind == kop.StatExtraLandPlay && st.ctrl == p && s.statCond(k, st) {
			add := int(st.a.Static.A)
			if add < 1 {
				add = 1
			}
			n += add
		}
		return true
	})
	if n > 255 {
		n = 255
	}
	return uint8(n)
}

// cantLose reports whether player p can't lose the game now (Herald of
// Eternal Dawn: "you can't lose" for its controller, "your opponents can't
// win" for its controller's opponent).
func (s *State) cantLose(k *prep, p uint8) bool {
	return s.cantLoseFor(k, p, 0)
}

// cantLoseLife is cantLose for the state-based loss at 0 or less life (CR
// 704.5a): it also honours a replacement that names only that reason.
func (s *State) cantLoseLife(k *prep, p uint8) bool {
	return s.cantLoseFor(k, p, kop.CantLoseLife)
}

// cantLoseFor reports whether a replacement stops player p losing for the
// reason (0: any other cause; kop.CantLoseLife): a Repl.C naming a reason
// applies only to that one.
func (s *State) cantLoseFor(k *prep, p uint8, reason int16) bool {
	if k.gen == nil {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		if st.a.Kind != kop.AbReplacement {
			return true
		}
		switch st.a.Repl.Kind {
		case kop.ReplCantLoseGame:
			hit = st.ctrl == p && (st.a.Repl.C == 0 || st.a.Repl.C == reason)
		case kop.ReplOppsCantWinGame:
			// the opponent of this player can't win: p (that opponent's
			// opponent) can't lose
			hit = st.ctrl == p
		}
		return !hit
	})
	return hit
}

// mayPlayStatics calls f for every StatMayPlayFrom static controlled by p
// that lets p play card object o (with the static's flags).
func (s *State) mayPlayStatics(k *prep, p uint8, o Obj, f func(st statSrc) bool) {
	ob := &s.objs[o]
	stopped := false
	check := func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatMayPlayFrom ||
			a.Static.StFlags&kop.StFreeOnly == 0 && !mayPlayFromZone(&a.Static, ob.zone) {
			return true
		}
		if a.Static.StFlags&kop.StSeatActive != 0 {
			// the beneficiary is the active player, whoever controls the static
			if p != s.active {
				return true
			}
			st.ctrl = p
		} else if st.ctrl != p {
			return true
		}
		if a.Static.StFlags&(kop.StFunctionsInGraveyard|kop.StFunctionsInExile) != 0 {
			return true // the card's own permission from a non-battlefield zone (below), not a permanent's
		}
		sctx := s.ctxStatic(st)
		if !s.statCond(k, st) || !s.matchCardGenAt(k, a.Static.Filter, ob.card, ob.owner(), p, o, &sctx) {
			return true
		}
		if a.Static.StFlags&kop.StOncePerTurn != 0 && s.usedThisTurn(st.obj, uint8(abIndex(k, s.cardAt(st.sl), a))) {
			return true
		}
		if !f(st) {
			stopped = true
			return false
		}
		return true
	}
	s.eachStaticC(k, sclsMayPlay, check)
	if stopped {
		// f stopped the walk at a permanent's permission: the card's own
		// graveyard permission is not offered again (Storied Veteran in the
		// graveyard under Leonardo, Sewer Samurai: gorge lists one cast)
		return
	}
	// a card's own "may cast this from the graveyard or exile" (Quilled
	// Greatwurm, Undead Sprinter's CheckSVar$ gate; Khan, Vengeful Heart):
	// the same condition and filter gates as a permanent's static, so a
	// self-grant whose condition does not hold offers nothing (gorge
	// mayPlayConditionGateHolds)
	var selfCls uint16
	var selfFlag kop.StaticFlags
	switch ob.zone {
	case kop.ZoneGraveyard:
		selfCls, selfFlag = sclsGyMay, kop.StFunctionsInGraveyard
	case kop.ZoneExile:
		selfCls, selfFlag = sclsExMay, kop.StFunctionsInExile
	default:
		return
	}
	if ob.owner() != p || k.gen.sCls[ob.card]&selfCls == 0 {
		return
	}
	cd := &k.t.Cards[ob.card]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatMayPlayFrom && a.Static.StFlags&selfFlag != 0 {
			st := statSrc{a: a, sl: NoSlot, ctrl: p, obj: o}
			sctx := s.ctxStatic(st)
			if !s.statCond(k, st) || !s.matchCardGenAt(k, a.Static.Filter, ob.card, ob.owner(), p, o, &sctx) {
				continue
			}
			if !f(st) {
				return
			}
		}
	}
}

// abIndex is a's index in card c's abilities.
func abIndex(k *prep, c CardID, a *kop.Ability) int {
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		if &cd.Abilities[i] == a {
			return i
		}
	}
	return 255
}

func (s *State) usedThisTurn(o Obj, ab uint8) bool {
	for i := 0; i < int(s.g.nused); i++ {
		if s.g.used[i].obj == o && s.g.used[i].ab == ab && (o >= MaxObj || s.g.used[i].ver == s.objs[o].ver) {
			return true
		}
	}
	return false
}

func (s *State) markUsed(o Obj, ab uint8) {
	if int(s.g.nused) < maxUsed {
		s.g.used[s.g.nused] = usedRec{obj: o, ab: ab, ver: s.objVer(o)}
		s.g.nused++
	} else {
		s.capFull(StallUsed)
	}
}

// preventCombatGen prevents the combat damage that would be dealt to a
// creature with KwPreventCombatDamageTo (Fleeting Flight), or one whose
// printed K:Prevent sentence stops combat damage to it (combat_prevention.go).
func (s *State) preventCombatGen(k *prep, r *dmgRec) {
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if r.toPerm[sl] > 0 && s.prevCombatTo(k, sl) {
			r.prevent(sl)
		}
	}
	if k.gen.anyPrevFrom {
		s.preventFromGen(k, r)
	}
}

// equipTargetPriced: the activation being built is an Equip ability whose
// price depends on its target (abilityHasTargetReduction: Cloud, Planet's
// Champion), so its target menu was pruned to the candidates the activator
// can pay for (abilityAffordableTargets).
func (s *State) equipTargetPriced(k *prep) (*kop.Ability, Slot, bool) {
	e := &s.bld().e
	if e.kind != seAbility || k.gen == nil || !k.gen.anyAbReduce {
		return nil, NoSlot, false
	}
	a := s.buildAbility(k)
	if a == nil || a.AFlags&kop.AfEquip == 0 {
		return nil, NoSlot, false
	}
	sl := s.srcSlot()
	if sl == NoSlot || !s.abilityHasTargetReduction(k, a, sl, e.ctrl) {
		return nil, NoSlot, false
	}
	return a, sl, true
}

// abilityAffordableTargets keeps, for an Equip ability whose cost reduction
// depends on its target (equipTargetPriced), only the candidates whose own
// price (the chosen earlier targets plus the candidate, after the ability's
// own ReduceCost$) the activator can pay; every candidate while a source the
// window census cannot price is open (as the spell branch).
func (s *State) abilityAffordableTargets(k *prep, buf *[MaxOptions]TCode, n int) int {
	a, sl, ok := s.equipTargetPriced(k)
	if !ok || DebugNoAbReduce {
		return n
	}
	e := &s.bld().e
	if s.windowUnpricedOpen(k, e) {
		return n
	}
	ti := min(int(s.bld().ti), kop.MaxTargets-1)
	var tg [kop.MaxTargets]TCode
	copy(tg[:ti], e.tgt[:ti])
	base := a.Cost.Mana
	if k.gen.anyOwnReduce && a.OwnReduce != (kop.ValueRef{}) {
		base = s.ownReducedBF(k, a, sl, e.ctrl)
	}
	var av avail
	s.availFor(k, e.ctrl, &av)
	m := 0
	for i := 0; i < n; i++ {
		tg[ti] = buf[i]
		c := s.abReducedFor(k, a, sl, e.ctrl, base, &tg, ti+1)
		if av.affords(&c, pipMaskOf(&c), 0) {
			buf[m] = buf[i]
			m++
		}
	}
	return m
}

// markCostComposition marks a per-target cost reprice of the in-flight cast, so
// the turn's cast counts hide its own ring push as gorge's costCompositionEvent
// does. Without it a "second spell you cast" reduction (Highspire Bell-Ringer)
// vanishes from the target menu while the build's own buildCost saw it.
//
// It reports whether it set the bit (the cast is a live spell build and the bit
// was clear); unmarkCostComposition(set) undoes exactly that.
func (s *State) markCostComposition() (set bool) {
	if !s.buildLive() || s.bld().e.kind != seSpell || s.bld().bfl&bflCostComp != 0 {
		return false
	}
	s.bld().bfl |= bflCostComp
	return true
}

// unmarkCostComposition clears the mark markCostComposition set.
func (s *State) unmarkCostComposition(set bool) {
	if set {
		s.bld().bfl &^= bflCostComp
	}
}

// affordableTargets keeps, for a spell whose cost reduction depends on its
// target (Luminous Rebuke), only the targets it can still pay for: all of
// them when the full cost is affordable, else those the reduction needs.
func (s *State) affordableTargets(k *prep, buf *[MaxOptions]TCode, n int) int {
	e := &s.bld().e
	if e.kind != seSpell {
		if k.gen.anyAbReduce {
			n = s.abilityAffordableTargets(k, buf, n) // a target-gated equip reduction
		}
		return n
	}
	if e.flags&sfFree != 0 {
		// a free cast: only a target-dependent raise prices it
		if k.gen.anyRaise {
			n = s.freeDropTargets(k, buf, n)
		}
		if k.gen.anyRaiseLife {
			n = s.raiseLifeDropTargets(k, buf, n)
		}
		return n
	}
	if k.gen.anyRaise {
		n = s.raiseDropTargets(k, buf, n) // a target-dependent raise (cost_reducespell.go)
	}
	if k.gen.anyRaiseLife {
		n = s.raiseLifeDropTargets(k, buf, n) // a target-dependent life raise (cost_raiselife.go)
	}
	cd := &k.t.Cards[e.card]
	var f kop.FilterID
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatReduceCost && a.Static.StFlags&kop.StTargetsFilter2 != 0 {
			f = a.Static.Filter2
		}
	}
	if f == 0 {
		return n
	}
	for i := 0; i < int(s.bld().ti) && i < kop.MaxTargets; i++ {
		if s.costTargetMatch(k, f, e.ctrl, e.tgt[i]) {
			// an earlier slot's target already satisfies the reduction, so the
			// cost is reduced whatever this slot picks (Run Over's second
			// target, the creature an opponent controls, once its first is a
			// Vehicle; gorge asks a later chain link's target unfiltered)
			return n
		}
	}
	s.g.noTgtRed = true
	marked := s.markCostComposition()
	full := s.adjustCostGen(k, e.ctrl, e.card, e.obj, cd.Cost)
	s.unmarkCostComposition(marked)
	s.g.noTgtRed = false
	if e.flags&sfAnyMana != 0 {
		full = anyType(full)
	}
	if e.flags&sfKicked != 0 {
		full = s.addKickerPayments(cd, e, full)
	}
	// The announced X is part of the cost the payment will charge; fold it
	// before pricing the full (unreduced) alternative, or a target that only
	// the reduction pays for looks affordable.
	full = full.AtX(int(e.x))
	var av avail
	// a granted ability that spends only on other payments (Clement's
	// creature-spell-only {G} or {U}) prices no candidate of this cast: gorge's
	// probe reads the window's units this payment may spend (CR 106.6)
	av.xd, av.xdOn = s.payDescNow(NoObj), true
	s.availForBuilt(k, e, &av)
	if av.affords(&full, pipMaskOf(&full), 0) {
		return n
	}
	if s.windowUnpricedOpen(k, e) {
		return n // a source the window census cannot price may close the gap: gorge keeps every candidate
	}
	m := 0
	for i := 0; i < n; i++ {
		if c := buf[i]; s.costTargetMatch(k, f, e.ctrl, c) {
			buf[m] = c
			m++
		}
	}
	return m
}
