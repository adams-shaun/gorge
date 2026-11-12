package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Lure (CR 509.1c, gorge kw:AllMustBlock, cr-lure-all-must-block
// 617daacfb): "All creatures able to block CARDNAME do so." gorge marks
// every offered block of a lure attacker a requirement of that blocker
// (askBlockers' BlockMust) and accepts only a declaration obeying the most
// requirements (decision.blockRequiredCore); the lockstep's completability
// then offers a duty blocker only its lure blocks and Done only once every
// duty blocker blocks a lure attacker. That maximum is every duty blocker
// here: each blocks one attacker, independently. Cold: reached only with
// prep.anyLure and a lure attacker in combat.

// A creature is a lure when its card prints the keyword (CardDef.Lure), when
// a live StatLure static applies to it (an Aura's enchanted creature, a
// threshold or colour condition, the creature itself), or when a Pump KW$
// grant marked it until end of turn (ExtMustBeBlocked with C =
// MustBeBlockedLure: the arena bit mechLure). Gorge reads the derived
// keyword list, so a loss of all abilities removes the printed keyword and
// the grants older than the loss; a grant made after it survives (CR 613.7).

// prepLure sets anyLure: some card is a lure, or the table can make one.
func prepLure(t *kop.Table, g *genPrep) {
	g.anyLure = g.ix[mechLure] != arenaNone || tableHasStatic(t, kop.StatLure)
	for i := 0; i < len(t.Cards) && !g.anyLure; i++ {
		if t.Cards[i].Lure {
			g.anyLure = true
			return
		}
	}
}

// lureMark is a Pump's lure grant on the permanent at sl: effective unless it
// has lost all abilities, in which case the grant postdates the loss and
// survives it.
func (s *State) lureMark(k *prep, sl Slot) {
	if !s.mSl(k, mechLure, sl) {
		s.mSlSet(k, mechLure, sl)
		s.touchSl(sl)
	}
	if !s.mSl(k, mechLurePost, sl) && s.genLost(k, sl) {
		s.mSlSet(k, mechLurePost, sl)
		s.touchSl(sl)
	}
}

// isLure reports whether the attacker at a is a lure now.
func (s *State) isLure(k *prep, a Slot) bool {
	if s.mSl(k, mechLurePost, a) {
		return true
	}
	if !s.genLost(k, a) && (k.t.Cards[s.cardAt(a)].Lure || s.mSl(k, mechLure, a)) {
		return true
	}
	if !k.gen.anyLureStat {
		return false
	}
	lts, lost := s.lossAt(k, a)
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		if ab := st.a; ab.Kind == kop.AbStatic && ab.Static.Kind == kop.StatLure && !(lost && s.grantTs(st) < lts) &&
			s.statCond(k, st) && s.matchPermGen(k, ab.Static.Filter, a, st.ctrl, st.sl, layerFull) {
			hit = true
			return false
		}
		return true
	})
	return hit
}

// lureAttackers is the attacking lure creatures.
func (s *State) lureAttackers(k *prep) (m slotMask) {
	for am := s.blockAtkM(k); am.any(); am = am.rest() {
		a := am.first()
		if s.perms[a].flags&pfAttacking != 0 && s.isLure(k, a) {
			m = m.with(a)
		}
	}
	return
}

// blockPairOK is blockerOptions' offer test for the pair (b, a).
func (s *State) blockPairOK(k *prep, b, a Slot) bool {
	return s.aloneBlockOfferable(k, b, a) && s.perms[a].flags&pfAttacking != 0 && s.canBlock(k, b, a) && !s.menaceImpossible(k, a) && s.blocksCompletable(k, b, a) &&
		!(k.gen != nil && k.gen.anyMaxBlk && s.blockCapped(k, a))
}

// blockerOptionsLure is blockerOptions with lure attackers in combat.
//
//go:noinline
func (s *State) blockerOptionsLure(k *prep, d *Decision, lure slotMask) bool {
	def := s.blockDef()
	owed := false
	for bm := s.ctrlM[def]; bm.any(); bm = bm.rest() {
		b := bm.first()
		if !s.blockerReady(k, b) {
			continue
		}
		duty := false
		for lm := lure; lm.any(); lm = lm.rest() {
			if s.blockPairOK(k, b, lm.first()) {
				duty = true
				break
			}
		}
		owed = owed || duty
		for am := s.blockAtkM(k); am.any(); am = am.rest() {
			a := am.first()
			if duty && !lure.has(a) || !s.blockPairOK(k, b, a) {
				continue
			}
			if int(d.N) >= MaxOptions-1 {
				return s.optionsFull(d)
			}
			d.add(MakeAction(AkBlock, uint8(s.perms[b].obj), uint8(s.perms[a].obj), 0))
		}
	}
	if !owed && s.blocksCompletable(k, NoSlot, NoSlot) && s.menaceDeficit(k, NoSlot, NoSlot, nil) == 0 && !s.loneAloneBlocker(k) {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	return true
}
