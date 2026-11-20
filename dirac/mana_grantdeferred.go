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
	return g.branch(t, i, sl, stepDeferred, t.deferMask(g.k, sl), 1, true) // an identity source asks its colour (mana_deferident.go)
}

// deferApply taps the deferred source of a found prefix step on s: its {R}
// floats as the deferred source's (floatDeferred), so a cast that spends it is
// paid by Goggles and its rider can fire.
func (s *State) deferApply(k *prep, p uint8, sl Slot, col int) {
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	s.floatDeferredAs(k, p, sl, col)
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

// availGrantCountedDeferred reports whether availForMode already counted the
// planner-deferred source sl through availGrant: under a continuous grant
// (Enduring Vitality's "creatures you control have {T}: Add one mana of any
// color") a deferred source's granted ability is a census unit of its own, and
// that unit is the source's one tap (CR 605.1a), so the X window's deferred
// census (availXWindow) must not add the rider's unit beside it. Twitching Doll
// under Vitality made one tap count twice and offered X one past gorge's.
func (s *State) availGrantCountedDeferred(k *prep, sl Slot) bool {
	return k.gen != nil && k.gen.anyContGrant && s.grantIdx(k, sl) >= 0
}
