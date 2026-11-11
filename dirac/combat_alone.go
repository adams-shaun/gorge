package dirac

// Printed "CARDNAME can't attack alone." (kop.Card.AloneAtk, CR 506.5;
// gorge combat.DeclaresAlone published as decision Option.AloneRestricted).
// gorge poses the attackers as one set and offers a pick only while some
// valid declaration still contains it, and Done only for a valid one: the
// kernel's pick-by-pick declaration offers a restricted creature only while
// another creature is declared or could be, and withholds Done while it is
// the lone attacker. A requirement to attack binds a lone restricted
// creature only while a partner could attack beside it (CR 508.1d; gorge
// requiredCore).

// DebugAloneAtkOff (test only, TestAtkaloneMutation): no creature is
// restricted, so a lone carrier attacks.
var DebugAloneAtkOff bool

// aloneAtk reports whether the creature at sl can't attack alone now.
func (s *State) aloneAtk(k *prep, sl Slot) bool {
	return k.gen != nil && k.gen.anyAloneAtk && !DebugAloneAtkOff && k.t.Cards[s.cardAt(sl)].AloneAtk && !s.genLost(k, sl)
}

// alonePartner reports whether another of the active player's creatures
// than sl is attacking or could be declared as an attacker.
func (s *State) alonePartner(k *prep, sl Slot) bool {
	for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
		q := m.first()
		if q != sl && (s.perms[q].flags&pfAttacking != 0 || s.canAttack(k, q)) {
			return true
		}
	}
	return false
}

// loneAloneAttacker reports whether exactly one creature is declared and it
// can't attack alone, so the declaration may not close.
func (s *State) loneAloneAttacker(k *prep) bool {
	if s.nAttackers != 1 || k.gen == nil || !k.gen.anyAloneAtk {
		return false
	}
	for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
		q := m.first()
		if s.perms[q].flags&pfAttacking != 0 {
			return s.aloneAtk(k, q)
		}
	}
	return false
}

// Printed "CARDNAME can't block alone." (kop.Card.AloneBlk, CR 506.5), and
// the block half of the compound "can't attack or block alone.": the
// creature may be declared as a blocker only beside another blocker in the
// same declaration. gorge poses the blocker pairs as one set and offers Done
// only for a valid answer (decision.Validate -> combat.DeclaresAlone), so the
// kernel mirrors the same two reads: a lone restricted pair is not offered
// when no partner can join it, and Done is withheld while the one declared
// blocker is restricted.

// DebugAloneBlkOff (test only): no creature is restricted from blocking
// alone, so a lone carrier blocks.
var DebugAloneBlkOff bool

// aloneBlk reports whether the creature at sl can't block alone now.
func (s *State) aloneBlk(k *prep, sl Slot) bool {
	return k.gen != nil && k.gen.anyAloneBlk && !DebugAloneBlkOff && k.t.Cards[s.cardAt(sl)].AloneBlk && !s.genLost(k, sl)
}

// blockDeclared reports whether this defender has already declared a block.
func (s *State) blockDeclared(k *prep) bool {
	def := s.blockDef()
	for bm := s.ctrlM[def]; bm.any(); bm = bm.rest() {
		if s.perms[bm.first()].flags&pfBlocking != 0 {
			return true
		}
	}
	return false
}

// blockPartnerPossible reports whether, with b declared blocking a, another
// ready creature of this defender could still be declared blocking some
// attacker and the declaration still complete (the offer test of
// combat_lure.go's blockPairOK with b's block committed: a's Max$ 1 cap
// counts b, menace and Min$ floors match with b used up; a restricted partner
// is a legal partner). gorge offers a pick only inside some valid
// declaration, so a partner b's own block shuts out is no partner.
func (s *State) blockPartnerPossible(k *prep, b, a Slot) bool {
	def := s.blockDef()
	maxBlk := k.gen != nil && k.gen.anyMaxBlk
	for bm := s.ctrlM[def]; bm.any(); bm = bm.rest() {
		q := bm.first()
		if q == b || !s.blockerReady(k, q) {
			continue
		}
		for am := s.blockAtkM(k); am.any(); am = am.rest() {
			a2 := am.first()
			if s.perms[a2].flags&pfAttacking == 0 || !s.canBlock(k, q, a2) || s.menaceImpossible(k, a2) {
				continue
			}
			if maxBlk && (s.blockCapped(k, a2) || a2 == a && s.maxOneBlocker(k, a)) {
				continue
			}
			if s.blocksCompletable2(k, b, a, q, a2) {
				return true
			}
		}
	}
	return false
}

// aloneBlockOfferable reports whether the pair (b blocks a) may be offered
// now: an unrestricted blocker always, a restricted one only when another
// block is already declared or can still join it (CR 506.5).
func (s *State) aloneBlockOfferable(k *prep, b, a Slot) bool {
	return !s.aloneBlk(k, b) || s.blockDeclared(k) || s.blockPartnerPossible(k, b, a)
}

// loneAloneBlocker reports whether exactly one blocker is declared and it
// can't block alone, so the declaration may not close.
func (s *State) loneAloneBlocker(k *prep) bool {
	if k.gen == nil || !k.gen.anyAloneBlk {
		return false
	}
	def := s.blockDef()
	lone := NoSlot
	n := 0
	for bm := s.ctrlM[def]; bm.any(); bm = bm.rest() {
		b := bm.first()
		if s.perms[b].flags&pfBlocking != 0 {
			lone = b
			n++
		}
	}
	return n == 1 && s.aloneBlk(k, lone)
}
