package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A wide second-ability gap (Hermitic Herbalist: {T}: one mana of any colour
// beside a restricted {T}: two mana in any combination of colours) beside a
// granted mana ability: gorge's census covers neither the combination nor the
// grant, so both sources are gaps, and its one prefix search
// (potential_script.go scriptSearch.visit) activates every census gap in
// battlefield order. A Herbalist entered before the grant's holder is tapped
// first, for its first ability's colour, though the grant and the printed
// sources alone would pay the play (CR 605.3a, 601.2g). So the grant search
// lists it as a candidate too, branching as the script search does
// (mana_restrictedcombo.go): the "choose a mana ability" ask, the first
// ability's colour ask, then the combination's allocation ask, one node each.

// stepWide is the gi of a prefix step that taps a wide second-ability gap; the
// step's col is the engine-order answer tapGapChoice decodes (wide2Answers).
const stepWide int16 = -5

// wideGaps is p's wide gaps: the second-ability ones, and the sources whose only
// ability is the wide one (Interdimensional Web Watch), which gorge's census
// misses the same way.
func (s *State) wideGaps(k *prep, p uint8) slotMask {
	var m slotMask
	if k.gen == nil || !k.gen.anyMultiR {
		return m
	}
	for b := s.scriptAnyR(k, p); b.any(); b = b.rest() {
		if sl := b.first(); s.wideSecond(k, sl) || s.wideOnly(k, sl) {
			m = m.with(sl)
		}
	}
	return m
}

// wideOnlyTaps: a wide-only source (its restricted ability is no census ability, so
// no canTapForMana profile) is untapped, keeps its abilities and is not
// summoning-sick (CR 302.6).
func (s *State) wideOnlyTaps(k *prep, sl Slot) bool {
	p := &s.perms[sl]
	return p.flags&pfTapped == 0 && p.ovr.flags&(ovLoseAll|ovLoseAllEOT) == 0 && !s.activationSick(k, sl)
}

// wideOnly: the source's only mana ability is the wide any-colour restricted one
// (restrWideAnyMana), so its activation poses no "choose a mana ability" ask.
func (s *State) wideOnly(k *prep, sl Slot) bool {
	c := s.cardAt(sl)
	j := k.gen.anyR(c)
	return j >= 0 && int(j) == firstManaAb(&k.t.Cards[c]) && restrWideAnyMana(&k.t.Cards[c].Abilities[j])
}

// wideBranch activates the wide candidate i (slot sl) when it can tap, by each
// answer of its asks in gorge's order.
func (g *grantSrch) wideBranch(t *State, i int, sl Slot) bool {
	if !t.canTapForMana(g.k, sl) && !(t.wideOnly(g.k, sl) && t.wideOnlyTaps(g.k, sl)) {
		return false
	}
	if t.wideSecond(g.k, sl) { // (a wide-only source has just the allocation ask below)
		if !g.node() { // the "choose a mana ability" ask
			return false
		}
		if !g.node() { // the first ability's colour ask
			return false
		}
		order := [kop.NumManaKinds - 1]uint8{0, 1, 2, 3, 4}
		needFirst(order[:], g.need)
		for _, c := range order {
			if g.wideTry(t, i, sl, c, false) {
				return true
			}
		}
	}
	return g.wideAlloc(t, i, sl)
}

// wideAlloc charges the wide ability's combination allocation ask and taps
// candidate i (slot sl) for each combination in engine order.
func (g *grantSrch) wideAlloc(t *State, i int, sl Slot) bool {
	if !g.node() { // the combination's allocation ask
		return false
	}
	ab := &g.k.t.Cards[t.cardAt(sl)].Abilities[g.k.gen.anyR(t.cardAt(sl))]
	skip := !t.restrSrcAdmits(g.k, g.p, sl, ab, &g.d)
	for c := 0; c < wideAnswers; c++ {
		if g.wideTry(t, i, sl, uint8(kop.NumManaKinds-1+wideEngineAnswer(g.need, c)), skip) {
			return true
		}
	}
	return false
}

// wide2Branch activates candidate i's wide second ability (slot sl) beside the
// usable printed first one it follows (Hermitic Herbalist bearing a grant):
// the "choose a mana ability" ask is the grant path's own, the first ability's
// colours its dual branch, so only the combination's allocation ask and the
// combinations in engine order are charged here.
func (g *grantSrch) wide2Branch(t *State, i int, sl Slot) bool {
	if !t.canTapForMana(g.k, sl) {
		return false
	}
	return g.wideAlloc(t, i, sl)
}

// wideTry taps candidate i for engine answer ans on a copy of t and continues
// the search there. skip: the combination's mana is past the play, so its units
// sit in the ghost (they price nothing, but gorge's state key reads them).
func (g *grantSrch) wideTry(t *State, i int, sl Slot, ans uint8, skip bool) bool {
	if g.np >= len(g.path) {
		return false
	}
	var gx [kop.NumManaKinds]uint8
	if skip {
		c1, c2 := wideAnswer(int(ans) - (kop.NumManaKinds - 1))
		gx[c1]++
		gx[c2]++
	}
	c := t.fork()
	c.perms[sl].flags |= pfTapped
	c.touchSl(sl)
	var spill [kop.NumManaKinds]uint8
	c.wide2Float(g.k, g.p, sl, int(ans), &spill)
	for j := range gx {
		g.ghost[j] += gx[j]
	}
	g.path[g.np] = grantStep{i: uint8(i), col: ans, gi: stepWide, sl: sl}
	g.np++
	ok := g.next(c, sl)
	if !ok {
		g.np--
	}
	for j := range gx {
		g.ghost[j] -= gx[j]
	}
	t.unfork()
	return ok
}

// wideApply taps the wide gap of a found prefix step on s.
func (s *State) wideApply(k *prep, p uint8, sl Slot, ans uint8) {
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	var spill [kop.NumManaKinds]uint8
	s.wide2Float(k, p, sl, int(ans), &spill)
}

// wide2Float floats engine answer c of the wide second-ability gap at sl: its
// first ability's colour (c below NumManaKinds-1), else the multiset of its
// combination, with the TapsForMana extra either tap brings (gorge
// addTapExtra). A unit the restricted table cannot hold is counted in spill.
func (s *State) wide2Float(k *prep, p uint8, sl Slot, c int, spill *[kop.NumManaKinds]uint8) {
	s.addExtra(k, p&1, sl, s.manaPCGen(k, sl))
	if c < kop.NumManaKinds-1 {
		s.pl[p].pool[c]++
		return
	}
	ab := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.anyR(s.cardAt(sl))]
	c1, c2 := wideAnswer(c - (kop.NumManaKinds - 1))
	for _, col := range [2]int{c1, c2} {
		if !s.floatRestrAb(k, p, sl, ab, col, 1) {
			spill[col]++
		}
	}
}

// wideBesideGrant reports whether p has an activatable wide-only gap (Interdimensional
// Web Watch) while a permanent of p's holds a live granted mana ability: the grant
// search is then gorge's one prefix search over both, and runs before the wide
// script, which would tap the Web Watch alone ahead of a granted source that
// precedes it in battlefield order.
func (s *State) wideBesideGrant(k *prep, p uint8) bool {
	if k.gen == nil || !k.gen.anyMultiR {
		return false
	}
	for b := s.scriptWide(k, p); b.any(); b = b.rest() {
		if s.wideOnly(k, b.first()) {
			return s.liveGrantGap(k, p)
		}
	}
	return false
}
