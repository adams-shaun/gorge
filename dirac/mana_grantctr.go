package dirac

// A counter-cost mana source (a storage land, Wall of Roots) beside a granted
// ability is a census gap of gorge's one prefix search, as a tap-another source
// is (mana_granttapx.go): rules scriptSearch.visit tries every gap in battlefield
// order, depth first, the granted ones and the counter-cost ones alike. A play
// only the pair pays (Mardu Banner's {R}{W}{B}, the {B} from Nature's Embrace's
// two mana, the {R} from a Mercadian Bazaar's counter) is therefore offered. The
// counter-cost search (tapXSearch) skips the grants, and a play the grant
// search refuses is never offered again (grantCap's capSpent), so without this
// branch the grant search dropped it.

// grantCtrGaps: p's counter-cost sources the grant search activates beside the
// grants: those whose activation asks nothing (a dead gap, a later ability
// beside a plain {T} first and an any-colour ability each ask, and stay with the
// counter-cost search alone).
func (s *State) grantCtrGaps(k *prep, p uint8) slotMask {
	var m slotMask
	if !k.ctrMana {
		return m
	}
	for b := s.scriptCtr(k, p); b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		if a := k.ctrAbOf(c); deadCtrAb(a) || k.ctrSecond(c) || ctrAnyColour(a) || a.ManaEach {
			continue
		}
		m = m.with(sl)
	}
	return m
}

// ctrBesideGrant reports whether p has a counter-cost gap the grant search
// activates while a permanent of p's holds a live granted mana ability: the
// grant search is then gorge's one prefix search over both, and an activation's
// prefix runs it before the counter-cost script (applyPriority), which would
// activate the counter-cost source alone and leave a granted source's tap to
// the payment.
func (s *State) ctrBesideGrant(k *prep, p uint8) bool {
	return s.grantCtrGaps(k, p).any() && s.liveGrantGap(k, p)
}

// ctrBranch activates candidate i's (slot sl) counter-cost ability on a copy
// of t: its counters, its use, its mana, with no ask. The step is a tap-another
// step with no permanent to tap (tapXDo's counter-cost arm).
func (g *grantSrch) ctrBranch(t *State, i int, sl Slot) bool {
	if !t.scriptCtr(g.k, g.p).has(sl) || g.np >= len(g.path) {
		return false
	}
	u := t.fork()
	u.tapXDo(g.k, g.p, tapXStep{sl: sl, cand: NoSlot, cand2: NoSlot, kind: tapXAct})
	g.path[g.np] = grantStep{i: uint8(i), gi: stepTapX, sl: sl, cand: NoSlot}
	g.np++
	var ok bool
	if g.tapsLive {
		ok = g.settle(u) // (the tap may have fired a trigger)
	} else {
		ok = g.visit(u)
	}
	t.unfork()
	if !ok {
		g.np--
	}
	return ok
}
