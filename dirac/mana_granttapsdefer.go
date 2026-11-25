package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A Taps-deferred mana source (a depletion land, Peat Bog: "{T}, Remove a
// depletion counter: Add {B}{B}", a bounded rider gorge's planner defers,
// mana_rider.go; a Taps watcher's deferred source, mana_tapsdefer.go) beside
// a granted mana ability (Abundant Growth on a Forest) or a
// tap-another-permanent source: gorge's census lists neither, and its one
// prefix search (potential_script.go potentialPrefixScript over census.gaps)
// activates every census gap in battlefield order, the deferred source among
// them. So the grant search (grantSrch) lists the deferred source as a gap
// candidate too (the tap-another search's is mana_tapxdefer.go), tapped as the
// script search taps it (scriptGapTap): a Forest, a Forest under Abundant
// Growth and a one-counter Peat Bog pay Fertile Footsteps' {2}{G}; Saprazzan
// Skerry's {U}{U} beside Gemhide Sliver's granted any-colour mana, or beside
// Birchlore Rangers's tap-two-Elves mana, pays Bloodrite Invoker's {8}
// (CR 601.2g-h, 605.1a, 605.3b).

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
	// The subsume run's colour ask keeps only the colours the pool still
	// lacks (subsumeKeeps, the pool read before the step's mana floats in):
	// the restricted census shape's answers after its plain float's place are
	// the restricted ability's colour ask (restrCensusDeferGap's order), and
	// a rider-only source's activation is its own colour ask. A rider beside
	// its plain first ability is one ask whose plain option names no symbol
	// (scriptManaUnits), and a source whose answers are fixed productions of
	// several abilities asks no colour: both asks stay whole. The plain run's
	// asks stay whole too (gorge's subsumeAnswers runs only in the subsume
	// one, scriptSearch.visit's sr.subsume gate): a prune here would drop the
	// play a plain run's tree within the budget prices, and grantSubsume then
	// refuses the un-limited refusal (TestGrantSubsumeTapsDeferPlainRun).
	var order [kop.NumManaKinds]uint8
	plain, prunes := 0, false
	if n > 1 {
		if t.restrCensusTapsGapShape(g.k, sl) {
			ab := &g.k.t.Cards[t.cardAt(sl)].Abilities[g.k.gen.restrRecOf(t.cardAt(sl)).ab]
			plain = 1
			prunes = g.subsume && n > 2 // the colour ask behind the float: more than one answer
			t.deferGapColourOrder(g.k, g.d.play, uint8(ab.ManaColors&kop.ColAny), &order)
		} else if rA, ok := t.riderAbilityOf(g.k, sl); ok && !t.riderPlainFirst(g.k, sl) && !rA.ManaEach {
			prunes = g.subsume
			t.deferGapRiderOrder(g.k, g.d.play, rA, &order)
		}
	}
	for c := uint8(0); c < n; c++ {
		if prunes && int(c) >= plain && !g.subsumeKeeps(t, order[c-uint8(plain)]) {
			continue // a covered colour pays nothing more, so the line through it is dominated
		}
		if g.tapsDeferTry(t, i, sl, c) {
			return true
		}
	}
	return false
}

// deferGapColourOrder is the colour ask's answers a Taps-deferred gap's
// activation walks for play a: the mask's colours W U B R G, the play's
// announced colours first (scriptEngineAnswer's and restrCensusDeferGap's
// needFirst hoist of scriptOrderAnswers).
func (s *State) deferGapColourOrder(k *prep, a Action, mask uint8, out *[kop.NumManaKinds]uint8) (n int) {
	for col := uint8(0); col < kop.NumManaKinds; col++ {
		if mask&(1<<col) != 0 {
			out[n] = col
			n++
		}
	}
	needFirst(out[:n], s.playNeed(k, a))
	return n
}

// deferGapRiderOrder is deferGapColourOrder for a rider ability: its colour
// ask lists a printed Combo's colours in printed order (riderAnswerOrder).
func (s *State) deferGapRiderOrder(k *prep, a Action, ab *kop.Ability, out *[kop.NumManaKinds]uint8) (n int) {
	n = riderAnswerOrder(ab, out)
	needFirst(out[:n], s.playNeed(k, a))
	return n
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
// mana and rider (riderGap), else the tap and its colour (tapDeferFloat). The
// script returns to priority after the step (as ctrGap's does), so state-based
// actions run.
func (s *State) tapsDeferApply(k *prep, p uint8, a Action, sl Slot, ans uint8) {
	var spill [kop.NumManaKinds]uint8
	s.scriptGapTap(k, p, a, sl, slotMask{}, slotMask{}, slotMask{}, slotMask{}.with(sl), slotMask{}, int(ans), &spill)
	s.sba(k)
}

// tdBesideGrant reports whether p has a Taps-deferred gap while a permanent of
// p's holds a live granted mana ability: the grant search is then gorge's one
// prefix search over both, and an activation's prefix runs it first
// (applyPriority).
func (s *State) tdBesideGrant(k *prep, p uint8) bool {
	return s.scriptTapsDefer(k, p).any() && s.liveGrantGap(k, p)
}
