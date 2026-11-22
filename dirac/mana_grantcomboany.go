package dirac

// A Combo Any source (Axebane Guardian's "{T}: Add X mana in any combination of
// colors") beside a granted mana ability (Citanul Hierophants' and Rishkar's
// "{T}: Add {G}", Abundant Growth's and Weirding Wood's any-colour land,
// Imprisoned in the Moon's {C}): gorge's census prices neither, so both are
// census gaps of the one prefix search (potential_script.go scriptSearch.visit,
// activations: every gap in battlefield order, the printed and granted
// abilities of one source as one "choose a mana ability" ask), and gorge's
// PotentialMana prices the Combo Any production open in every colour
// (potential.go addPotentialMana), so the walk lists every play (caPotentialOpen)
// and each takes that search. The grant search lists the Combo Any source as a
// gap candidate of its own (grantSrch.ca): a holder of a grant that is a Combo
// Any source too branches its printed ability's allocation answers before the
// grants' (the printed ability first in gorge's ask, CR 605.3a), and one that
// holds none taps as a gap alone. The colour ask is the one the Combo Any chain
// answers (caActivate, caRank: gorge's scriptAnswers in odometer order, the
// colours the play names first). A source with a filter beside the grants and
// the Combo Any source, a tap-another source, a reflected one or a Diamond keeps
// the table refused (prepComboAnyMix, StallComboAnyMix).

// stepComboAny is the gi of a prefix step that taps a Combo Any gap; the step's
// col is the odometer answer (caActivate's ans).
const stepComboAny int16 = -12

// DebugComboAnyGrantNoGap (mutation test only): the grant search leaves the Combo
// Any source out of its candidates, so its mana never joins the prefix.
var DebugComboAnyGrantNoGap bool

// caGrantGaps are p's Combo Any gap sources the grant search takes as candidates.
func (s *State) caGrantGaps(k *prep, p uint8) slotMask {
	if DebugComboAnyGrantNoGap || k.gen == nil || !k.gen.anyComboAny {
		return slotMask{}
	}
	return s.caGaps(k, p)
}

// caBesideGrant reports whether p has a live Combo Any source while a permanent
// of p's holds a live granted mana ability: the grant search is then gorge's one
// prefix search over both, run before the script search (grantFirstGaps).
func (s *State) caBesideGrant(k *prep, p uint8) bool {
	return s.caGrantGaps(k, p).any() && s.liveGrantGap(k, p)
}

// caOpenGrant reports whether gorge's potential walk is open beside the grants:
// a live Combo Any source prices an unbounded production in every colour
// (caPotentialOpen), so every play the walk lists takes the prefix search.
func (s *State) caOpenGrant(k *prep, p uint8) bool {
	return s.caPotentialOpen(k, p) && s.liveGrantGap(k, p)
}

// caStep reports whether the found prefix taps a Combo Any gap.
func (g *grantSrch) caStep() bool {
	for i := 0; i < g.np; i++ {
		if g.path[i].kind == stepAct && g.path[i].gi == stepComboAny {
			return true
		}
	}
	return false
}

// caGapBranch activates the Combo Any gap of candidate i (slot sl) when it is
// still untapped and makes mana: one colour-ask node, then the allocation's
// answers in gorge's order (caRank), each continuing the search.
func (g *grantSrch) caGapBranch(t *State, i int, sl Slot) bool {
	if !t.caGaps(g.k, g.p).has(sl) || !g.node() {
		return false
	}
	units := t.caAmount(g.k, sl)
	for r, na := 0, caAnswerCount(units); r < na; r++ {
		if g.caTry(t, i, sl, caRank(units, &g.need, r)) {
			return true
		}
	}
	return false
}

// caTry taps candidate i for odometer answer ans on a copy of t and continues
// the search there.
func (g *grantSrch) caTry(t *State, i int, sl Slot, ans int) bool {
	if g.np >= len(g.path) {
		return false
	}
	c := t.fork()
	c.caProduce(g.k, g.p, sl, ans)
	g.path[g.np] = grantStep{i: uint8(i), col: uint8(ans), gi: stepComboAny, sl: sl}
	g.np++
	ok := g.next(c, sl)
	if !ok {
		g.np--
	}
	t.unfork()
	return ok
}

// caProduce is caActivate without the Taps notification (the search's next
// stacks it) and the tap's extra stall (declared where it is replayed).
func (s *State) caProduce(k *prep, p uint8, sl Slot, ans int) {
	n := s.caAmount(k, sl)
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	var got [5]int
	for u := max(n-3, 0); u < n; u++ {
		got[caDigit(ans, n, u)]++
	}
	got[0] += max(n-3, 0)
	for col, c := range got {
		if c > 0 {
			s.poolAdd(p, col, c)
			s.artTag(k, p, sl, col)
		}
	}
	s.tapExtraStall(k, sl)
}

// caGrantRefused reports whether the table's granted mana abilities are of a
// shape the grant search does not take a Combo Any source beside: a spend
// restriction, or a grant whose step costs life or a sacrifice. The table's
// filters are candidates of the same search (grantSrch.filt). Plain "{T}: Add {G}" or any-colour grants (Citanul
// Hierophants, Rishkar, Abundant Growth, Imprisoned in the Moon) are chained.
// DebugComboAnyGrantRefused (mutation test only) refuses every grant, as before.
func caGrantRefused(g *genPrep) bool {
	if DebugComboAnyGrantRefused {
		return true
	}
	for i := range g.grantPC {
		if g.grantRestr[i] != 0 || grantHasCost(&g.grantPC[i]) {
			return true
		}
	}
	return false
}

// DebugComboAnyGrantRefused (mutation test only): a Combo Any source beside a
// granted mana ability is refused at the table's start, as before the chain.
var DebugComboAnyGrantRefused bool

// eachOrComboAnyApply replays the each-colour or Combo Any gap step st on s.
func (s *State) eachOrComboAnyApply(k *prep, p uint8, sl Slot, st *grantStep) {
	if st.gi == stepComboAny {
		s.caActivate(k, p, sl, int(st.col))
		GrantManaStats.ComboAny.Add(1)
		return
	}
	s.eachApply(k, p, sl, st.col)
}

// grantWalkMode is the census mode of grantCap's potential walk beside a live
// grant: gorge's walk is not relaxable with a source that grants its own
// colours (availHyp), and folds open beside a storage battery or a Combo Any
// source (availAny: mana_storeopen.go, mana_grantcomboany.go).
func (s *State) grantWalkMode(k *prep, p uint8) uint16 {
	m := availWalk
	if s.selfGrantLive(k, p) {
		m |= availHyp // no relaxed proof: gorge's census is not relaxable (mana_owncolours.go)
	}
	if s.storeOpenLive(k, p) || s.caOpenGrant(k, p) {
		m |= availAny
	}
	return m
}
