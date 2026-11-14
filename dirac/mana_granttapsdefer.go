package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A Taps-deferred mana source (a depletion land, Peat Bog: "{T}, Remove a
// depletion counter: Add {B}{B}", a bounded rider gorge's planner defers,
// mana_rider.go; a Taps watcher's deferred source, mana_tapsdefer.go) beside
// a granted mana ability (Abundant Growth on a Forest): gorge's census lists
// neither, and its one prefix search (potential_script.go
// potentialPrefixScript over census.gaps) activates every census gap in
// battlefield order, the deferred source among the grants. So the grant
// search (grantSrch) lists the deferred source as a gap candidate too, tapped
// as the script search taps it (scriptGapTap): a Forest, a Forest under
// Abundant Growth and a one-counter Peat Bog pay Fertile Footsteps' {2}{G},
// which the grant search alone dropped (CR 601.2g-h, 605.3b).

// stepTapsDefer is the gi of a prefix step that activates a Taps-deferred gap;
// the step's col is the script search's answer index (scriptGapTap).
const stepTapsDefer int16 = -7

// tapsDeferGrantGaps is p's Taps-deferred gaps the grant search lists: the
// script search's (scriptTapsDefer), for a play the search prices.
func (s *State) tapsDeferGrantGaps(k *prep, p uint8, d *payDesc) slotMask {
	if !d.hasPlay {
		return slotMask{}
	}
	return s.scriptTapsDefer(k, p)
}

// tapsDeferBranch activates the Taps-deferred candidate i (slot sl) when it can
// still be activated, by each answer of its colour ask in the script search's
// order (deferGapAnswers; one answer poses no ask node).
func (g *grantSrch) tapsDeferBranch(t *State, i int, sl Slot) bool {
	if !t.occ.has(sl) || t.perms[sl].obj != g.cobj[i] || !t.tapsDeferGrantGaps(g.k, g.p, &g.d).has(sl) {
		return false // tapped, spent or sacrificed by an earlier step
	}
	n := t.deferGapAnswers(g.k, sl)
	if n > 1 && !g.node() {
		return false
	}
	for c := uint8(0); c < n; c++ {
		if g.tapsDeferTry(t, i, sl, c) {
			return true
		}
	}
	return false
}

// tapsDeferTry activates candidate i for answer ans on a copy of t and
// continues the search there.
func (g *grantSrch) tapsDeferTry(t *State, i int, sl Slot, ans uint8) bool {
	if g.np >= len(g.path) {
		return false
	}
	c := t.fork()
	c.tapsDeferApply(g.k, g.p, g.d.play, sl, ans)
	g.path[g.np] = grantStep{i: uint8(i), col: ans, gi: stepTapsDefer, sl: sl}
	g.np++
	ok := g.next(c, sl)
	if !ok {
		g.np--
	}
	t.unfork()
	return ok
}

// tapsDeferApply activates the Taps-deferred gap sl for play a's answer ans,
// as the script search's prefix does (scriptGapTap): a rider's counters, tap,
// mana and rider (riderGap), else the tap and its colour (tapDeferFloat).
func (s *State) tapsDeferApply(k *prep, p uint8, a Action, sl Slot, ans uint8) {
	var spill [kop.NumManaKinds]uint8
	s.scriptGapTap(k, p, a, sl, slotMask{}, slotMask{}, slotMask{}, slotMask{}.with(sl), int(ans), &spill)
}
