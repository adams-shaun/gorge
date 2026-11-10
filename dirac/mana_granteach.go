package dirac

// An each-colour gap (Tarnation Vista's "{1}, {T}: For each colour among
// monocoloured permanents you control, add one mana of that colour", a Bloom
// Tender's free {T}) beside a granted mana ability (Lotus Ring's "Sacrifice
// this creature: Add three mana of any one color"): gorge's census covers
// neither the each-colour ability nor the grant, so both sources are gaps, and
// its one prefix search (potential_script.go scriptSearch.visit) activates
// every census gap in battlefield order. A Vista entered before the Ring's
// bearer is tapped first, for its chosen colour, though the Ring alone would
// pay the play (CR 605.3a, 601.2g). So the grant search lists the each-colour
// gap as a candidate too, branching as eachSearch does (mana_eachfee.go): the
// chosen {T} first, then the fee-paid ability when the pool holds its fee, one
// ask node when both are offered.

// stepEach is the gi of a prefix step that taps an each-colour gap; the
// step's col is the ability taken (eachProduce's ans).
const stepEach int16 = -6

// eachBranch activates the each-colour candidate i (slot sl) when it is still
// untapped, by each ability its activation offers.
func (g *grantSrch) eachBranch(t *State, i int, sl Slot) bool {
	if t.perms[sl].flags&pfTapped != 0 {
		return false
	}
	fee := t.eachFee(g.k, sl)
	if fee == 0 {
		return g.eachTry(t, i, sl, 0)
	}
	chosen, paid := t.eachChosenMask(g.k, sl) != 0, t.eachFeePayable(g.k, g.p, sl, fee)
	switch {
	case chosen && paid:
		// the activation asks which ability; the chosen {T} is listed first
		if !g.node() {
			return false
		}
		return g.eachTry(t, i, sl, 0) || g.eachTry(t, i, sl, 1)
	case chosen:
		return g.eachTry(t, i, sl, 0)
	case paid:
		return g.eachTry(t, i, sl, 1)
	}
	return false
}

// eachTry taps candidate i by ability ans on a copy of t and continues the
// search there.
func (g *grantSrch) eachTry(t *State, i int, sl Slot, ans uint8) bool {
	if g.np >= len(g.path) {
		return false
	}
	c := t.fork()
	c.perms[sl].flags |= pfTapped
	c.touchSl(sl)
	c.eachProduce(g.k, g.p, sl, ans)
	g.path[g.np] = grantStep{i: uint8(i), col: ans, gi: stepEach, sl: sl}
	g.np++
	ok := g.next(c, sl)
	if !ok {
		g.np--
	}
	t.unfork()
	return ok
}

// eachApply taps the each-colour gap of a found prefix step on s.
func (s *State) eachApply(k *prep, p uint8, sl Slot, ans uint8) {
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	s.eachProduce(k, p, sl, ans)
}

// eachBesideGrant reports whether p has an activatable each-colour gap while a
// permanent of p's holds a live granted mana ability: the grant search is then
// gorge's one prefix search over both, and runs before the each-colour script.
func (s *State) eachBesideGrant(k *prep, p uint8) bool {
	return s.scriptGaps(k, p).any() && s.liveGrantGap(k, p)
}
