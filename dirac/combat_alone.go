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
