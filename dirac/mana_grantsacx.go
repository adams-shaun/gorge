package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A sacrifice-another mana source (kop.SacXMana: Krark-Clan Ironworks'
// "Sacrifice an artifact: Add {C}{C}", kop.SacAnyManaAb, or Bolg's Company's
// "{T}, Sacrifice another Goblin: Add {B}{R}", kop.SacXEachManaAb) beside a
// granted mana ability (Abundant Growth on a land): gorge's census defers the
// sacrifice part (a Sac part that is not the source) and its one prefix
// search (potential_script.go scriptSearch.visit) activates every census gap
// in battlefield order, the sacrifice source and the granted sources among
// them, so a cast only their mana together pays (Brood Monitor's {4}{G}{G}
// from the {C}{C}, the granted {G} and two lands) is found through it. Neither
// kernel search alone prices that play: the tap-another search's walk runs
// without the grants (mana_tapanother.go tapXScript's noGrant), and the grant
// search (grantSrch) left the sacrifice sources to it (mana_granttapx.go).
// So the grant search lists the sacrifice source as a candidate of its own
// (grantSacXGaps), the way it lists the tap-another one (mana_granttapx.go)
// and the Taps-deferred one (mana_granttapsdefer.go): the permanent to
// sacrifice (pay.ManaSacrificeCandidates' ask, only with two or more
// candidates, battlefield order; the source itself a candidate of
// kop.SacAnyManaAb, Krark-Clan Ironworks among them), then the colour of an
// any-colour production (kop.SacXAnyColour) or its fixed production with no
// colour ask (tapXUnit), as the tap-another search's sacrifice branch does
// (mana_tapanother.go sacStep). A grant holder whose own
// ability is a sacrifice-another shape stays on the grant side: its granted
// abilities' ask lists its printed abilities first, the sacrifice among them
// (printedXFirst; Phyrexian Tower under Abundant Growth: {C}, the sacrifice,
// then the grant).

// stepSacX is the gi of a prefix step that activates a candidate's
// sacrifice-another mana ability (cand the permanent it sacrifices, col the
// colour an any-colour production was asked).
const stepSacX int16 = -10

// grantSacXGaps: p's sacrifice-another mana sources the tap-another search
// models (scriptTapXModelled), a slot mask, for a play the grant search
// prices. tapXOffGrant excludes them (the grant search covers a source the
// tap-another search alone would keep, mana_granttapx.go).
func (s *State) grantSacXGaps(k *prep, p uint8) slotMask {
	var m slotMask
	if !k.tapMana {
		return m
	}
	for b := s.scriptTapXModelled(k, p); b.any(); b = b.rest() {
		sl := b.first()
		if kop.SacXMana(k.tapXAbOf(s.cardAt(sl))) {
			m = m.with(sl)
		}
	}
	return m
}

// sacXBesideGrant: p has a live sacrifice-another mana source beside a live
// granted mana ability: gorge's one prefix search activates both, so a play
// the tap-another search alone cannot pay takes the grant search
// (script_gen.go scriptOffers).
func (s *State) sacXBesideGrant(k *prep, p uint8) bool {
	return s.grantSacXGaps(k, p).any() && s.liveGrantGap(k, p)
}

// sacXBranch activates candidate i's (slot sl) sacrifice-another mana ability
// on copies of t: a second-ability source's ability ask first (its plain {T}
// ability the first answer, Phyrexian Tower's {C}: sacXPlain), then each
// permanent its cost may sacrifice (the ask only with two or more candidates,
// battlefield order), then the colour of an any-colour production (Phyrexian
// Altar, Gilded Goose: W U B R G with the play's announced colours first, as
// gorge's scriptOrderAnswers orders the ask); a fixed production asks none.
func (g *grantSrch) sacXBranch(t *State, i int, sl Slot) bool {
	if !g.sacXLive(t, i, sl) {
		return false
	}
	if found, stop := g.sacXPlain(t, i, sl); stop {
		return found
	}
	return g.sacXAct(t, i, sl)
}

// sacXLive: candidate i (slot sl) is still on the battlefield and its
// sacrifice-another ability has a permanent to sacrifice.
func (g *grantSrch) sacXLive(t *State, i int, sl Slot) bool {
	if !t.occ.has(sl) || t.perms[sl].obj != g.cobj[i] {
		return false // sacrificed by an earlier step (mana_grantcost.go)
	}
	return g.k.tapXAbOf(t.cardAt(sl)) != nil && t.scriptTapX(g.k, g.p).has(sl) // (tapped or its candidates gone since the root)
}

func (g *grantSrch) sacXLast(t *State, sl Slot, cs *[MaxPerm]Slot, anyCol bool) bool {
	cost := g.cost
	if g.dyn {
		cost = t.grantDynamicCost(g.k, g.p, g.d.play, g.full)
	}
	var cols [5]uint8
	ncol := 0
	if fc, ok := kop.TapXFixedColour(g.k.tapXAbOf(t.cardAt(sl))); ok {
		cols[0], ncol = uint8(fc), 1
	} else if anyCol {
		for i := range cols {
			cols[i] = uint8(i)
		}
		ncol = 5
	} else {
		cols[0], ncol = 0, 1
	}
	saved := g.cheaper
	for i := 0; i < ncol; i++ {
		u := t.fork()
		u.tapXDo(g.k, g.p, tapXStep{sl: sl, cand: cs[0], cand2: NoSlot, col: cols[i], kind: tapXAct})
		var d Decision
		ok := u.tapXNext(g.p, &d) && d.Kind == DkPriority && g.leafPays(u, &cost, saved)
		t.unfork()
		if ok {
			g.cheaper = saved
			return true
		}
	}
	g.cheaper = saved
	return false
}

// planFirst is the grant search's root probe for a play the plain-creature
// sacrifice sources pay with no census gap beside them: gorge's planner
// (autoPayOffer's last-resort arm, pay.PlanOtherSacShape) funds it before the
// prefix search is asked for any gap, so no granted ability's {T} floats
// beside it. One source's single activation is walked first, ahead of the gaps
// before it; else the sources activate once each in gorge's plan order
// (planOtherSacSourceOrder), two or more, the fewest that pay, each naming the
// plan's victim (planUnits). Any other node, and a play needing a gap's unit
// first, stay the battlefield-order walk.
func (g *grantSrch) planFirst(t *State, sacd bool, stepped *slotMask) (found, stop bool) {
	if g.np != 0 || g.gapN < 2 {
		return false, false
	}
	var src [MaxPerm]int
	m := 0
	for i := 0; i < g.gapN; i++ {
		sl := g.cand[i]
		if !g.sacx[i] || !g.sacXLive(t, i, sl) || !t.planOtherSacSource(g.k, sl) {
			continue
		}
		src[m] = i
		m++
	}
	for j := 0; j < m; j++ {
		i := src[j]
		sl := g.cand[i]
		ab := g.k.tapXAbOf(t.cardAt(sl))
		var cs [MaxPerm]Slot
		if t.sacXCands(g.k, g.p, sl, ab, &cs) == 0 || !g.sacXLast(t, sl, &cs, kop.SacXAnyColour(ab)) {
			continue
		}
		if i == 0 {
			return false, false // already the walk's first gap
		}
		return g.gapVisit(t, i, sacd, stepped)
	}
	cost := g.cost
	if g.dyn {
		cost = t.grantDynamicCost(g.k, g.p, g.d.play, g.full)
	}
	for k := 2; k <= m; k++ {
		if g.planUnits(t, src[:k], 0, &cost) {
			return true, false
		}
	}
	return false, false
}

// planUnits activates src[j:]'s sacrifice sources once each on copies of t,
// every one sacrificing the victim gorge's plan names (sacXVictimOrder), each
// colour answer of an any-colour source tried, and prices the play once the
// last one has produced. A found line stays in g.path.
func (g *grantSrch) planUnits(t *State, src []int, j int, cost *kop.ManaCost) bool {
	if j == len(src) {
		saved := g.cheaper
		if g.leafPays(t, cost, saved) {
			return true
		}
		g.cheaper = saved
		return false
	}
	i := src[j]
	sl := g.cand[i]
	ab := g.k.tapXAbOf(t.cardAt(sl))
	var cs [MaxPerm]Slot
	nc := t.sacXCands(g.k, g.p, sl, ab, &cs)
	if nc == 0 || !t.occ.has(sl) || t.perms[sl].obj != g.cobj[i] {
		return false
	}
	var vbuf [MaxPerm]Slot
	v := t.sacXVictimOrder(g.k, sl, &cs, nc, true, &vbuf)
	if kop.SacReflManaAb(ab) {
		// The plan cannot price the victim's own symbols, so the search tries
		// every candidate the sacrifice ask offers, with that victim's
		// producible colours in script-answer order. An unproductive land is
		// the activation that adds nothing: gorge's search still takes it.
		for ci := 0; ci < nc; ci++ {
			var order [kop.NumManaKinds]uint8
			ncol := sacReflAnswers(t.sacReflMask(g.k, v[ci]), g.need, &order)
			if ncol == 0 {
				ncol = 1 // no colour ask; the activation adds nothing
			}
			for _, col := range order[:ncol] {
				if g.np >= len(g.path) {
					return false
				}
				u := t.fork()
				u.tapXDo(g.k, g.p, tapXStep{sl: sl, cand: v[ci], cand2: NoSlot, col: col, kind: tapXAct})
				g.path[g.np] = grantStep{i: uint8(i), col: col, gi: stepSacX, sl: sl, cand: v[ci]}
				g.np++
				ok := g.planUnits(u, src, j+1, cost)
				t.unfork()
				if ok {
					return true
				}
				g.np--
			}
		}
		return false
	}
	order, ncol := [5]uint8{0, 1, 2, 3, 4}, 1
	if fc, ok := kop.TapXFixedColour(ab); ok {
		order[0] = uint8(fc)
	} else if kop.SacXAnyColour(ab) {
		ncol = 5
		needFirst(order[:], g.need)
	} else {
		order[0] = 0
	}
	for _, col := range order[:ncol] {
		if g.np >= len(g.path) {
			return false
		}
		u := t.fork()
		u.tapXDo(g.k, g.p, tapXStep{sl: sl, cand: v[0], cand2: NoSlot, col: col, kind: tapXAct})
		g.path[g.np] = grantStep{i: uint8(i), col: col, gi: stepSacX, sl: sl, cand: v[0]}
		g.np++
		ok := g.planUnits(u, src, j+1, cost)
		t.unfork()
		if ok {
			return true
		}
		g.np--
	}
	return false
}

// sacXAct is sacXBranch's activation of the sacrifice-another ability: the
// sacrifice ask, then the colour ask of an any-colour production.
func (g *grantSrch) sacXAct(t *State, i int, sl Slot) bool {
	ab := g.k.tapXAbOf(t.cardAt(sl))
	var cs [MaxPerm]Slot
	nc := t.sacXCands(g.k, g.p, sl, ab, &cs)
	if nc == 0 {
		return false
	}
	if nc > 1 && !g.node() { // the sacrifice ask
		return false
	}
	anyCol := kop.SacXAnyColour(ab)
	plan := t.planOtherSacSource(g.k, sl) && g.sacXLast(t, sl, &cs, anyCol)
	var vbuf [MaxPerm]Slot
	vorder := t.sacXVictimOrder(g.k, sl, &cs, nc, plan, &vbuf)
	refl := kop.SacReflManaAb(ab)
	// the colour ask's answers: W U B R G, the play's announced colours first
	// (gorge's scriptOrderAnswers, needFirst); a fixed production asks none
	order, ncol := [kop.NumManaKinds]uint8{0, 1, 2, 3, 4, 5}, 1
	if anyCol {
		ncol = 5
		needFirst(order[:ncol], g.need)
	}
	for j := 0; j < nc; j++ {
		if refl {
			// The victim's own symbols are the colour ask: one answer per
			// symbol in script order, none for an unproductive land (the
			// activation still resolves, adding nothing).
			ncol = sacReflAnswers(t.sacReflMask(g.k, vorder[j]), g.need, &order)
			if ncol == 0 {
				ncol = 1
			}
		}
		if (anyCol || refl) && ncol > 1 && !g.node() { // the colour ask
			return false
		}
		for _, col := range order[:ncol] {
			if g.np >= len(g.path) {
				return false
			}
			u := t.fork()
			u.tapXDo(g.k, g.p, tapXStep{sl: sl, cand: vorder[j], cand2: NoSlot, col: col, kind: tapXAct})
			g.path[g.np] = grantStep{i: uint8(i), col: col, gi: stepSacX, sl: sl, cand: vorder[j]}
			g.np++
			sv := g.refreshGrants(u) // (a sacrificed grant holder's grants are gone below the step: mana_grantlive.go)
			ok := g.next(u, sl)      // (the sacrifice queues its dies triggers, mana_granttapsdefer.go)
			g.restoreGrants(&sv)
			t.unfork()
			if ok {
				return true
			}
			g.np--
		}
	}
	return false
}

// sacXPlain is sacXBranch's ability ask of a second-ability source
// (kop.SecondSacTapManaAb, Phyrexian Tower): one node, its plain {T} ability
// tried first, as the tap-another search's tapXAbilityAsk does. stop:
// sacXBranch returns found (an answer paid, or the budget ran out on the ask),
// else it goes on to the sacrifice ability.
func (g *grantSrch) sacXPlain(t *State, i int, sl Slot) (found, stop bool) {
	if !g.k.tapXSecond(t.cardAt(sl)) || !t.canTapForMana(g.k, sl) {
		return false, false
	}
	if !g.node() { // the ability ask
		return false, true
	}
	ppc := t.manaPCGen(g.k, sl)
	if g.branch(t, i, sl, -1, t.srcMaskAt(ppc, sl), ppc.manaN, false) {
		return true, true
	}
	return false, false
}

// printedXFirst is visit's branch of a grant holder's printed tap- or
// sacrifice-another ability, the answers gorge's "choose a mana ability" ask
// lists before the grants: a second-ability source's plain {T} ability first
// (mana_tapanother.go tapXPlainDo; Phyrexian Tower's {C} under Chromatic
// Lantern), then the tap-another ability (mana_granttapx.go) or the
// sacrifice-another one (Gilded Goose under Cryptolith Rite: no plain ability,
// its sacrifice the ask's first answer).
func (g *grantSrch) printedXFirst(t *State, i int, sl Slot) bool {
	if g.k.tapXSecond(t.cardAt(sl)) && g.dual[i] && t.canTapForMana(g.k, sl) {
		// (a no-{T} second ability, Relic of Legends', stays live once its plain
		// {T} ability has tapped the source: that answer is gone)
		ppc := t.manaPCGen(g.k, sl)
		if g.branch(t, i, sl, -1, t.srcMaskAt(ppc, sl), ppc.manaN, false) {
			return true
		}
	}
	if g.sacx[i] {
		return g.sacXLive(t, i, sl) && g.sacXAct(t, i, sl)
	}
	return g.tapXBranch(t, i, sl)
}
