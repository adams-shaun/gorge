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

// prepLure sets anyLure: some card is a lure.
func prepLure(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		if t.Cards[i].Lure {
			g.anyLure = true
			return
		}
	}
}

// lureAttackers is the attacking lure creatures (abilities not lost).
func (s *State) lureAttackers(k *prep) (m slotMask) {
	for am := s.blockAtkM(k); am.any(); am = am.rest() {
		a := am.first()
		if s.perms[a].flags&pfAttacking != 0 && k.t.Cards[s.cardAt(a)].Lure && !s.genLost(k, a) {
			m = m.with(a)
		}
	}
	return
}

// blockPairOK is blockerOptions' offer test for the pair (b, a).
func (s *State) blockPairOK(k *prep, b, a Slot) bool {
	return s.perms[a].flags&pfAttacking != 0 && s.canBlock(k, b, a) && !s.menaceImpossible(k, a) && s.blocksCompletable(k, b, a) &&
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
	if !owed && s.blocksCompletable(k, NoSlot, NoSlot) && s.menaceDeficit(k, NoSlot, NoSlot, nil) == 0 {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	return true
}
