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

// lureFloorMet reports whether a non-lure attacker a with a block floor (menace's
// two, a Min$) still has that many free blockers when b blocks it: a duty
// blocker is committed to a lure (gorge's most-requirements rule rejects
// pulling it off), so only the blockers without a lure pair count, beside the
// blockers a already has. Without a floor above one it is vacuously met.
func (s *State) lureFloorMet(k *prep, duty slotMask, b, a Slot) bool {
	need := s.minBlockers(k, a)
	if need < 2 {
		return true
	}
	n := 1 + int(s.perms[a].nblock) // b itself, and the creatures already declared blocking a
	for bm := s.ctrlM[s.blockDef()]; bm.any(); bm = bm.rest() {
		b2 := bm.first()
		if b2 != b && !duty.has(b2) && s.blockerReady(k, b2) && s.blockPairOK(k, b2, a) {
			n++
		}
	}
	return n >= need
}

// blockFloorInCombat reports whether an attacking creature carries a block
// floor above one (menace's two, a MinMaxBlocker Min$, combat.go
// minBlockers): an attacker blocked below it pulls blockers, which is what
// can starve the lure requirement of a spilled duty blocker.
func (s *State) blockFloorInCombat(k *prep) bool {
	for am := s.blockAtkM(k); am.any(); am = am.rest() {
		if a := am.first(); s.perms[a].flags&pfAttacking != 0 && s.minBlockers(k, a) >= 2 {
			return true
		}
	}
	return false
}

// lureMaxSat is the most duty blockers the lure attackers can host: a lure
// attacker under a Max$ 1 static (combat_maxblockers.go) takes one further
// blocker, the others any number, and every assignment needs blockPairOK.
// CR 509.1c's duty binds a blocker only while the lure attacker is still
// able to take it: past the capacity the surplus blockers are free (gorge's
// quota -- its requiredQuota is the maximum team), so gorge's completability
// offers their plain pairs. The kernel mirrors that with this maximum.
func (s *State) lureMaxSat(k *prep, duty, lure slotMask) int {
	var single slotMask // lure attackers that host one blocker at most
	for lm := lure; lm.any(); lm = lm.rest() {
		if s.maxOneBlocker(k, lm.first()) {
			single = single.with(lm.first())
		}
	}
	big := lure.andNot(single)
	free, rest := 0, slotMask{} // duty an unbounded lure attacker takes / the leftovers
	for dm := duty; dm.any(); dm = dm.rest() {
		b := dm.first()
		taken := false
		for bm := big; bm.any(); bm = bm.rest() {
			if s.blockPairOK(k, b, bm.first()) {
				taken = true
				break
			}
		}
		if taken {
			free++
		} else {
			rest = rest.with(b)
		}
	}
	if free >= duty.count() {
		return duty.count()
	}
	return free + s.lureMatch(k, rest, single)
}

// lureMatch is the maximum bipartite matching of duty blockers onto
// one-blocker lure attackers (each edge a blockPairOK pair).
func (s *State) lureMatch(k *prep, duty, slots slotMask) int {
	var match [MaxPerm]Slot // slot -> its blocker, NoSlot for none
	for i := range match {
		match[i] = NoSlot
	}
	n := 0
	for dm := duty; dm.any(); dm = dm.rest() {
		var seen slotMask
		if s.lureAugment(k, dm.first(), slots, &match, &seen) {
			n++
		}
	}
	return n
}

// lureAugment is one augmenting-path round of lureMatch.
func (s *State) lureAugment(k *prep, b Slot, slots slotMask, match *[MaxPerm]Slot, seen *slotMask) bool {
	for sm := slots; sm.any(); sm = sm.rest() {
		sl := sm.first()
		if seen.has(sl) || !s.blockPairOK(k, b, sl) {
			continue
		}
		seen.set(sl)
		if match[sl] == NoSlot || s.lureAugment(k, match[sl], slots, match, seen) {
			match[sl] = b
			return true
		}
	}
	return false
}

// blockerOptionsLure is blockerOptions with lure attackers in combat.
//
//go:noinline
func (s *State) blockerOptionsLure(k *prep, d *Decision, lure slotMask) bool {
	def := s.blockDef()
	var dutyM slotMask
	for bm := s.ctrlM[def]; bm.any(); bm = bm.rest() {
		b := bm.first()
		if !s.blockerReady(k, b) {
			continue
		}
		for lm := lure; lm.any(); lm = lm.rest() {
			if s.blockPairOK(k, b, lm.first()) {
				dutyM = dutyM.with(b)
				break
			}
		}
	}
	// The surplus: when the lure attackers cannot host every duty blocker,
	// a duty blocker whose removal keeps the maximum is free -- it may be
	// the one the capacity leaves out (CR 509.1c's "if able"), so its plain
	// pairs stay offerable (gorge's completability search).
	maxSat := s.lureMaxSat(k, dutyM, lure)
	var spill slotMask
	if maxSat < dutyM.count() {
		for dm := dutyM; dm.any(); dm = dm.rest() {
			b := dm.first()
			if s.lureMaxSat(k, dutyM.without(b), lure) == maxSat {
				spill = spill.with(b)
			}
		}
	}
	// The spill's plain pairs are not all offerable: committing a spilled
	// duty blocker to a non-lure attacker under a block floor (menace's two,
	// a Min$, CR 509.1b) can leave the floor to be filled by another duty
	// blocker, which drops the declaration below the lure maximum -- the
	// two readings (the floor is coverable, the duty blocker is free) cannot
	// hold together. gorge's completability search rejects such a pair, so
	// the offer goes to the requirement search (blockerOptionsReq), whose
	// reachability is that same completion test over both at once. Cold: a
	// lure attacker short of its duty, with a floored attacker in combat.
	if spill.any() && s.blockFloorInCombat(k) {
		return s.blockerOptionsReq(k, d, slotMask{})
	}
	owed := dutyM.any()
	for bm := s.ctrlM[def]; bm.any(); bm = bm.rest() {
		b := bm.first()
		if !s.blockerReady(k, b) {
			continue
		}
		duty := dutyM.has(b)
		for am := s.blockAtkM(k); am.any(); am = am.rest() {
			a := am.first()
			if duty && !lure.has(a) && !spill.has(b) {
				continue
			}
			if !s.blockPairOK(k, b, a) {
				continue
			}
			if !lure.has(a) && !spill.has(b) && !s.lureFloorMet(k, dutyM, b, a) {
				continue
			}
			if int(d.N) >= MaxOptions-1 {
				return s.optionsFull(d)
			}
			d.add(MakeAction(AkBlock, TCode(s.perms[b].obj), TCode(s.perms[a].obj), 0))
		}
	}
	if !owed && s.blocksCompletable(k, NoSlot, NoSlot) && s.menaceDeficit(k, NoSlot, NoSlot, nil) == 0 && !s.loneAloneBlocker(k) {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	return true
}
