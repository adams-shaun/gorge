package dirac

// A deferred mana source (Pyromancer's Goggles' TriggersWhenSpent$ {R},
// mana_deferred.go) beside a granted mana ability (New Horizons on a land):
// gorge's census lists neither (pay.PlanHasSpecialProductionParam defers the
// one, PaymentAbility excludes the other), and its one prefix search
// (potential_script.go potentialPrefixScript) activates every census gap in
// battlefield order, the deferred source among the grants. So the grant
// search (grantSrch) lists the deferred source as a gap candidate too: a
// Mountain under New Horizons for {R}{R} plus Goggles' {R} and three printed
// lands pay Lathliss, Dragon Queen's {4}{R}{R} (CR 601.2g-h, 605.1a).

// stepDeferred is the gi of a prefix step that taps a deferred source for its
// {R}.
const stepDeferred int16 = -4

// deferBranch taps the deferred candidate i (slot sl) for its mana when it is
// still untapped: a single-ability source, so no ask node.
func (g *grantSrch) deferBranch(t *State, i int, sl Slot) bool {
	if t.perms[sl].flags&pfTapped != 0 {
		return false
	}
	return g.branch(t, i, sl, stepDeferred, 1<<g.k.deferCol(t.cardAt(sl)), 1, false)
}

// deferApply taps the deferred source of a found prefix step on s: its {R}
// floats as the deferred source's (floatDeferred), so a cast that spends it is
// paid by Goggles and its rider can fire.
func (s *State) deferApply(k *prep, p uint8, sl Slot) {
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	s.floatDeferred(k, p, sl)
}

// deferBesideGrant reports whether p has an untapped deferred source while a
// permanent of p's holds a granted mana ability. gorge's one prefix search
// activates both census gaps in battlefield order (potential_script.go
// scriptSearch.visit), and the grant search is that search (its candidates
// list the deferred source), so applyPriority runs it before the scripted-gap
// prefix, which would tap the deferred source alone: Storm Fleet Spy under
// Imprisoned in the Moon ({T}: Add {C}) entered before Pyromancer's Goggles,
// and gorge taps it first for Shivan Dragon's {R} pump, then the Goggles
// (CR 601.2g, 602.2b).
func (s *State) deferBesideGrant(k *prep, p uint8) bool {
	if !s.scriptDeferred(k, p).any() {
		return false
	}
	for m := s.ctrlM[SeatIx(p)].and(s.occ); m.any(); m = m.rest() {
		var gi [maxGrants]int16
		if s.grantIdxs(k, m.first(), &gi) > 0 {
			return true
		}
	}
	return false
}
