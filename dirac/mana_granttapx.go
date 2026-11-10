package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A tap-another-permanent source beside a granted mana ability (Gene
// Pollinator's "{T}, Tap an untapped permanent you control: Add one mana of
// any color" under Mm'menon, the Right Hand's granted {U}). gorge's census
// misses both (potential_plan.go paymentPlanCensusOf), and its prefix search
// (rules scriptSearch.visit) activates every gap it misses in priority-option
// order, so a play only the tap-another source pays (Mechanozoa's {4}{U}{U}
// from hand, which the granted {U} may not fund) is found through it. The
// grant search (grantSrch) branched over the grants alone and dropped that
// play; it now lists the tap-another source as a candidate whose printed
// ability is tried first: the permanent to tap (an ask with two or more
// candidates, battlefield order), then the colour (W U B R G), as tapXSearch
// does (mana_tapanother.go). A second tap-another ability beside a plain
// first (Scene of the Crime) is a candidate too: its ability ask lists the
// plain {C} first, then the tap-another ability. A two-permanent cost (Baylen)
// is not modelled here: it stays out of the grant search, priced by the
// tap-another search alone (tapXOffGrant).

// stepTapX is the gi of a prefix step that activates a candidate's
// tap-another-permanent ability (cand the permanent it taps, col its colour).
const stepTapX int16 = -3

// grantTapXGaps: p's one-permanent tap-another sources on a board whose
// census gaps the tap-another search models (scriptTapXModelled).
func (s *State) grantTapXGaps(k *prep, p uint8) slotMask {
	var m slotMask
	if !k.tapMana {
		return m
	}
	for b := s.scriptTapXModelled(k, p); b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		if a := k.tapXAbOf(c); a.Cost.N == 1 && a.Cost.Flags&kop.CostTap != 0 && !kop.SacXEachManaAb(a) { // (a sacrifice-another source is the tap-another search's alone)
			m = m.with(sl)
		}
	}
	return m
}

// tapXBranch activates candidate i's (slot sl) tap-another ability on copies
// of t: each permanent its cost may tap, then each colour.
func (g *grantSrch) tapXBranch(t *State, i int, sl Slot) bool {
	var cs [MaxPerm]Slot
	nc := t.tapXCands(g.k, g.p, sl, &cs)
	if nc > 1 && !g.node() { // the creature ask
		return false
	}
	for j := 0; j < nc; j++ {
		if nc > 1 && tapXOwnAnswer(g.ex, cs[j], NoSlot) {
			continue // it would tap the play's own {T} source (CR 602.2b)
		}
		if !g.node() { // the colour ask
			return false
		}
		for col := uint8(0); col < 5; col++ {
			if g.np >= len(g.path) {
				return false
			}
			u := t.fork()
			u.tapXDo(g.k, g.p, tapXStep{sl: sl, cand: cs[j], cand2: NoSlot, col: col, kind: tapXAct})
			g.path[g.np] = grantStep{i: uint8(i), col: col, gi: stepTapX, sl: sl, cand: cs[j]}
			g.np++
			var ok bool
			if g.tapsLive {
				ok = g.settle(u) // (the taps may have fired a trigger)
			} else {
				ok = g.visit(u)
			}
			t.unfork()
			if ok {
				return true
			}
			g.np--
		}
	}
	return false
}

// tapXOffGrant: p has a live tap-another source the grant search does not
// activate (a two-permanent cost, Baylen): only the tap-another search prices
// a play it pays. A source the grant search covers is tried in gorge's one
// prefix search beside the grants, whose budget the granted gaps spend
// (rules scriptSearch.visit), so the tap-another search alone, which skips
// the grants, must not keep a play that search gave up on.
func (s *State) tapXOffGrant(k *prep, p uint8) bool {
	return s.scriptTapXModelled(k, p).andNot(s.grantTapXGaps(k, p)).any()
}

// tapXSecondLive: p has a live tap-another source that is a later ability
// beside a plain {T} first (Scene of the Crime), which the grant search
// activates beside the grants.
func (s *State) tapXSecondLive(k *prep, p uint8) bool {
	for b := s.grantTapXGaps(k, p); b.any(); b = b.rest() {
		if k.tapXSecond(s.cardAt(b.first())) {
			return true
		}
	}
	return false
}
