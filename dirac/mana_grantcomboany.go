package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A Combo Any source (Axebane Guardian's "{T}: Add X mana in any combination of
// colors") beside a granted mana ability (Citanul Hierophants' and Rishkar's
// "{T}: Add {G}", Abundant Growth's and Weirding Wood's any-colour land,
// Imprisoned in the Moon's {C}): gorge's census prices neither, so both are
// census gaps of the one prefix search (potential_script.go scriptSearch.visit,
// activations: every gap in battlefield order, the printed and granted
// abilities of one source as one "choose a mana ability" ask), and gorge's
// PotentialMana prices the Combo Any production open in its colours
// (potential.go addPotentialMana), so the walk lists every play (caOpenGrant)
// and each takes that search. The grant search lists the Combo Any source as a
// gap candidate of its own (grantSrch.ca): a holder of a grant that is a Combo
// Any source too branches its printed ability's allocation answers before the
// grants' (the printed ability first in gorge's ask, CR 605.3a), and one that
// holds none taps as a gap alone. The colour ask is the one the Combo Any chain
// answers (caActivate, caRankMask: gorge's scriptAnswers in odometer order over
// the printed colours, the ones the play names first). A finite-mask source
// (Vivi Ornitier's {U}/{R}) opens the potential walk in its own faces alone and
// beside another ability asks no allocation: gorge flattens it into one option
// per colour (caFlatBranch). A source with a filter beside the grants and the
// Combo Any source, a tap-another source, a reflected one or a Diamond keeps
// the table refused (prepComboAnyMix, StallComboAnyMix).

// stepComboAny is the gi of a prefix step that taps a Combo Any gap; the step's
// col is the odometer answer (caActivate's ans).
const stepComboAny int16 = -12

// stepComboOne is the gi of a prefix step that activates a finite-mask Combo
// source through its flattened ability option (caFlatBranch); the step's col
// is the colour.
const stepComboOne int16 = -13

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
// a live Combo source prices an unbounded production in its colours
// (caWalkOpen), so every play the walk lists takes the prefix search, which
// keeps only the plays its prefix pays.
func (s *State) caOpenGrant(k *prep, p uint8) bool {
	return s.caWalkOpen(k, p) && s.liveGrantGap(k, p)
}

// caStep reports whether the found prefix taps a Combo Any gap.
func (g *grantSrch) caStep() bool {
	for i := 0; i < g.np; i++ {
		if g.path[i].kind == stepAct && (g.path[i].gi == stepComboAny || g.path[i].gi == stepComboOne) {
			return true
		}
	}
	return false
}

// caGapBranch activates the Combo Any gap of candidate i (slot sl) when it is
// still untapped and makes mana: one colour-ask node, then the allocation's
// answers in gorge's order (caRank), each continuing the search. A reflected
// Amount$ source's ask is its reflected candidates in gorge's order instead
// (reflAmtOrder, the play's announced colours first; mana_reflamt.go), the
// whole evaluated amount floating in the answer's colour.
func (g *grantSrch) caGapBranch(t *State, i int, sl Slot) bool {
	if !t.caGaps(g.k, g.p).has(sl) || !g.node() {
		return false
	}
	if g.k.isReflAmt(t.cardAt(sl)) {
		return g.reflGapBranch(t, i, sl)
	}
	units, mask := t.caAmount(g.k, sl), t.caMask(g.k, sl)
	for r, na := 0, caAnswerCountMask(units, mask); r < na; r++ {
		if g.caTry(t, i, sl, caRankMask(units, mask, &g.need, r)) {
			return true
		}
	}
	return false
}

// reflGapBranch is caGapBranch for a reflected Amount$ source (Incubation
// Druid): the whole evaluated amount floats in one of the reflected colours,
// the ones the play announces first (reflAmtOrderNeed); no colour at all is
// the tap alone (mana_reflamt.go).
func (g *grantSrch) reflGapBranch(t *State, i int, sl Slot) bool {
	var order [6]uint8
	cols := t.reflAmtOrderNeed(g.k, sl, g.need, order[:])
	if len(cols) == 0 {
		cols = append(order[:0], reflAmtNoMana)
	}
	for _, c := range cols {
		if g.caTry(t, i, sl, int(c)) {
			return true
		}
	}
	return false
}

// caTry activates candidate i for odometer answer ans on a copy of t and
// continues the search there.
func (g *grantSrch) caTry(t *State, i int, sl Slot, ans int) bool {
	if g.np >= len(g.path) {
		return false
	}
	c := t.fork()
	c.caProduce(g.k, g.p, sl, ans)
	return g.caStepped(c, grantStep{i: uint8(i), col: uint8(ans), gi: stepComboAny, sl: sl})
}

// caStepped records step st, already taken on the fork c, and continues the
// search below it, dropping both again.
func (g *grantSrch) caStepped(c *State, st grantStep) bool {
	g.path[g.np] = st
	g.np++
	ok := g.caNext(c, st.sl)
	if !ok {
		g.np--
	}
	c.unfork() // releases c, the latest fork
	return ok
}

// caNext is next after a Combo step on sl: a step that tapped nothing (Vivi's
// {0} ability) fires no Taps listener, so only what the step itself queued
// settles before the next node.
func (g *grantSrch) caNext(t *State, sl Slot) bool {
	if t.caTapsSource(g.k, sl) {
		return g.next(t, sl)
	}
	sv := g.refreshGrants(t)
	defer g.restoreGrants(&sv)
	if t.npend == 0 {
		return g.visit(t)
	}
	u := t.fork()
	ok := g.settle(u)
	t.unfork()
	return ok
}

// caFlatBranch branches a finite-mask Combo source that stands beside another
// mana ability (Vivi Ornitier holding a granted {T}: Add {G}): gorge's "choose
// a mana ability" ask flattens the printed Combo into one option per colour
// (pay.ManaAbilityComboColours), and the answer resolves the ability with
// Produced$ rewritten to that colour, so the whole amount floats in it with no
// allocation ask (CR 605.3a). The options name their colour, so pass sel walks
// the ones the play needs first (selColours; 0 walks all, needed first).
func (g *grantSrch) caFlatBranch(t *State, i int, sl Slot, sel uint8) bool {
	if !t.caGaps(g.k, g.p).has(sl) {
		return false
	}
	mask := uint8(t.caMask(g.k, sl))
	var order [kop.NumManaKinds]uint8
	n := t.stepColours(g.k, sl, -1, mask, &order)
	needFirst(order[:n], g.need)
	cols := order[:n]
	if sel != 0 {
		cols = g.selColours(cols, mask, sel, true)
	}
	for _, col := range cols {
		if g.np >= len(g.path) {
			return false
		}
		c := t.fork()
		c.caPayCost(g.k, sl)
		c.caFloat(g.k, g.p, sl, int(col))
		if g.caStepped(c, grantStep{i: uint8(i), col: col, gi: stepComboOne, sl: sl}) {
			return true
		}
	}
	return false
}

// comboPathPaid reports whether candidate i is a source gorge's search no
// longer activates (potential_script.go pathPaidTap): this line already
// activated it, it stands untapped (Vivi's {0}) with its once-each-turn Combo
// spent, and every ability left on it costs {T}. The other order stays open: a
// source its granted {T} tapped still takes its {0}.
func (g *grantSrch) comboPathPaid(t *State, i int, sl Slot) bool {
	if !g.ca[i] || t.perms[sl].flags&pfTapped != 0 || t.caTapsSource(g.k, sl) || t.caGaps(g.k, g.p).has(sl) {
		return false
	}
	for j := 0; j < int(g.ng[i]); j++ {
		if t.grantView(g.k.gen, g.gi[i][j], sl).grantSac {
			return false // a granted ability with no {T} is left
		}
	}
	for _, st := range g.path[:g.np] {
		if st.kind == stepAct && st.sl == sl {
			return true
		}
	}
	return false
}

// comboAloneStep is gapVisit's step for a Combo source holding a grant it
// cannot activate now (its granted {T} spent, or a summoning-sick holder)
// while its own cost needs no {T} (Vivi Ornitier's {0}): the one ability
// offered, so no ability ask, its allocation alone (CR 605.3a, 302.6). A
// source with no grant branches as its gap (gapBranch).
func (g *grantSrch) comboAloneStep(t *State, i int, sl Slot) bool {
	return g.ca[i] && g.ng[i] > 0 && t.caGaps(g.k, g.p).has(sl) && g.caGapBranch(t, i, sl)
}

// comboHolderBranch is gapAbilities' printed Combo step in the pass g.sel
// walks, before the grants (afterGrants false) or after them. A Combo Any
// ability's option names no colour, so it waits for pass 2 and its allocation
// is asked after. A flattened finite-mask one beside another ability
// (caFlattened, alts >= 2) names its colours: it walks each pass ahead of the
// grants, except that a granted fixed-colour ability gorge hoists
// (grantHoisted) walks pass 1 first. Alone it asks its allocation.
func (g *grantSrch) comboHolderBranch(t *State, i int, sl Slot, alts int, afterGrants bool) bool {
	if !g.ca[i] || !t.caGaps(g.k, g.p).has(sl) {
		return false
	}
	if alts < 2 || !t.caFlattened(g.k, sl) {
		return !afterGrants && g.sel != 1 && g.caGapBranch(t, i, sl)
	}
	hoisted := false
	for j := 0; g.sel == 1 && j < int(g.ng[i]); j++ {
		mask := t.srcMaskAt(t.grantView(g.k.gen, g.gi[i][j], sl), sl)
		hoisted = hoisted || g.grantHoisted(t, sl, g.gi[i][j], mask)
	}
	if hoisted != afterGrants {
		return false
	}
	sel := g.sel
	g.sel = 0 // the search below is whole again (branch)
	ok := g.caFlatBranch(t, i, sl, sel)
	g.sel = sel
	return ok
}

// caFlattened reports whether the Combo source at sl is one gorge's ability
// ask flattens into colour options: a finite mask of two colours or more.
func (s *State) caFlattened(k *prep, sl Slot) bool {
	if k.isReflAmt(s.cardAt(sl)) || !k.isComboAny(s.cardAt(sl)) {
		return false
	}
	mask := s.caMask(k, sl)
	return mask != kop.ColAny && mask&(mask-1) != 0
}

// caFloat adds the Combo source's whole amount in colour col (a flattened
// ability option's answer).
func (s *State) caFloat(k *prep, p uint8, sl Slot, col int) {
	s.poolAdd(p, col, s.caAmount(k, sl))
	s.artTag(k, p, sl, col)
	s.tapExtraStall(k, sl)
}

// caProduce is caActivate without the Taps notification (the search's next
// stacks it) and the tap's extra stall (declared where it is replayed). A
// reflected Amount$ source's whole evaluated amount floats in the answer's
// colour (mana_reflamt.go reflAmtDo's shape); no candidates means no mana.
func (s *State) caProduce(k *prep, p uint8, sl Slot, ans int) {
	if k.isReflAmt(s.cardAt(sl)) {
		s.reflAmtTap(k, p, sl, ans, false) // ans is the colour (mana_reflamt.go)
		return
	}
	n := s.caAmount(k, sl)
	s.caPayCost(k, sl)
	s.caAdd(k, p, sl, ans, n)
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

// eachOrComboAnyStep replays the each-colour or Combo Any gap step st on the
// chain's state and reports whether it tapped its source into triggers. A
// finite Combo that taps nothing (Vivi's {0}) fires none; that is read before
// the step is applied.
func (s *State) eachOrComboAnyStep(k *prep, g *grantSrch, sl Slot, st *grantStep) bool {
	taps := st.gi == stepEach || s.caTapsSource(k, sl)
	s.eachOrComboAnyApply(k, g.p, sl, st) // (mana_granteach.go)
	return taps && g.tapTriggers(s, sl)
}

// eachOrComboAnyApply replays the each-colour or Combo Any gap step st on s.
func (s *State) eachOrComboAnyApply(k *prep, p uint8, sl Slot, st *grantStep) {
	if st.gi == stepComboOne {
		s.caTap(k, sl)
		s.caFloat(k, p, sl, int(st.col))
		GrantManaStats.ComboAny.Add(1)
		return
	}
	if st.gi == stepComboAny {
		if k.isReflAmt(s.cardAt(sl)) {
			s.reflAmtDo(k, p, sl, int(st.col)) // the colour (mana_reflamt.go)
			GrantManaStats.ComboAny.Add(1)
			return
		}
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
