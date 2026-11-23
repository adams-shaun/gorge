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
// plain {C} first, then the tap-another ability. A two-permanent cost
// (Supportive Parents's "Tap two untapped creatures you control: Add one mana
// of any color", Baylen) is a candidate too: its pair of permanents is an ask
// with more than two candidates (every pair in lexicographic order, 64 at
// most, tapXPairAt), then the colour, as the tap-another search's pairAsk does.

// stepTapX is the gi of a prefix step that activates a candidate's
// tap-another-permanent ability (cand the permanent it taps, col its colour).
const stepTapX int16 = -3

// grantTapXGaps: p's one- and two-permanent tap-another sources (Urza's
// no-{T} "Tap an untapped artifact you control: Add {U}" among them) on a board whose
// census gaps the tap-another search models (scriptTapXModelled). A
// sacrifice-another source is not one: beside a granted mana ability the
// grant search activates it as a candidate of its own (grantSacXGaps,
// mana_grantsacx.go), the way gorge's one prefix search does.
func (s *State) grantTapXGaps(k *prep, p uint8) slotMask {
	var m slotMask
	if !k.tapMana {
		return m
	}
	for b := s.scriptTapXModelled(k, p); b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		a := k.tapXAbOf(c)
		if kop.SacXMana(a) || kop.ExileGManaAb(a) { // (an exile-from-graveyard source's picks are graveyard indices: mana_exilegrave.go)
			continue
		}
		if a.Cost.N == 1 || a.Cost.N == 2 { // (a one-permanent cost with or without {T}: Urza's has none)
			m = m.with(sl)
		}
	}
	return m
}

// tapXBranch activates candidate i's (slot sl) tap-another ability on copies
// of t: each permanent its cost may tap, then each colour, the play's
// colours first (tapXColours; none asked of a fixed production).
func (g *grantSrch) tapXBranch(t *State, i int, sl Slot) bool {
	var cs [MaxPerm]Slot
	nc := t.tapXCands(g.k, g.p, sl, &cs)
	if g.k.tapXAbOf(t.cardAt(sl)).Cost.N == 2 {
		return g.tapXPairBranch(t, i, sl, &cs, nc)
	}
	if nc > 1 && !g.node() { // the creature ask
		return false
	}
	order, ncol := g.tapXColours(t, sl)
	for j := 0; j < nc; j++ {
		if tapXOwnAnswer(g.ex, cs[j], NoSlot) {
			// it would tap the play's own {T} source (CR 602.2b); a forced
			// single answer too, as tapXSearch skips it: the play could not
			// be paid after it, and the node pricing, which reads no tapped
			// state, would otherwise call it payable (Nephalia Smuggler's
			// {3}{U},{T} whose only tap-another candidate is itself)
			continue
		}
		if ncol > 1 && !g.node() { // the colour ask
			return false
		}
		for _, col := range order[:ncol] {
			if g.np >= len(g.path) {
				return false
			}
			u := t.fork()
			u.tapXDo(g.k, g.p, tapXStep{sl: sl, cand: cs[j], cand2: NoSlot, col: col, kind: tapXAct})
			g.path[g.np] = grantStep{i: uint8(i), col: col, gi: stepTapX, sl: sl, cand: cs[j], cand2: NoSlot}
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

// tapXColours is the answers of tapXBranch's colour ask for source sl: W U B
// R G with the play's announced colours first (gorge's scriptOrderAnswers on
// that ask, needFirst), or one fixed colour and no ask (Urza's {U},
// kop.TapXFixedColour).
func (g *grantSrch) tapXColours(t *State, sl Slot) ([5]uint8, int) {
	if fc, ok := kop.TapXFixedColour(g.k.tapXAbOf(t.cardAt(sl))); ok {
		return [5]uint8{uint8(fc)}, 1
	}
	order := [5]uint8{0, 1, 2, 3, 4}
	needFirst(order[:], g.need)
	return order, 5
}

// tapXPairBranch is tapXBranch for a two-permanent cost: two candidates are
// tapped with no ask, more are asked as one "choose 2" (every pair of its
// candidates, tapXPairLimit), then the colour.
func (g *grantSrch) tapXPairBranch(t *State, i int, sl Slot, cs *[MaxPerm]Slot, nc int) bool {
	if nc > 2 && !g.node() { // the pair ask
		return false
	}
	order := [5]uint8{0, 1, 2, 3, 4}
	needFirst(order[:], g.need) // (gorge's scriptOrderAnswers on the colour ask)
	for pi, npair := 0, tapXPairLimit(nc); pi < npair; pi++ {
		x, y := tapXPairAt(nc, pi)
		a, b := cs[x], cs[y]
		if tapXOwnAnswer(g.ex, a, b) {
			continue // it would tap the play's own {T} source (CR 602.2b)
		}
		if !g.node() { // the colour ask
			return false
		}
		for _, col := range order {
			if g.np >= len(g.path) {
				return false
			}
			u := t.fork()
			u.tapXDo(g.k, g.p, tapXStep{sl: sl, cand: a, cand2: b, col: col, kind: tapXAct})
			g.path[g.np] = grantStep{i: uint8(i), col: col, gi: stepTapX, sl: sl, cand: a, cand2: b}
			g.np++
			var ok bool
			if g.tapsLive {
				ok = g.settle(u)
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
// activate (a cost of three or more permanents): only the tap-another search prices
// a play it pays. A source the grant search covers is tried in gorge's one
// prefix search beside the grants, whose budget the granted gaps spend
// (rules scriptSearch.visit), so the tap-another search alone, which skips
// the grants, must not keep a play that search gave up on. A
// sacrifice-another source is covered too (grantSacXGaps, mana_grantsacx.go).
func (s *State) tapXOffGrant(k *prep, p uint8) bool {
	return s.scriptTapXModelled(k, p).andNot(s.grantTapXGaps(k, p)).andNot(s.grantSacXGaps(k, p)).any()
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

// tapXPairLive: p has a live two-permanent tap-another source (Supportive
// Parents), which the grant search activates beside the grants.
func (s *State) tapXPairLive(k *prep, p uint8) bool {
	for b := s.grantTapXGaps(k, p); b.any(); b = b.rest() {
		if k.tapXAbOf(s.cardAt(b.first())).Cost.N == 2 {
			return true
		}
	}
	return false
}
