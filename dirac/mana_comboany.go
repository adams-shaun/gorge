package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Axebane Guardian: "{T}: Add X mana in any combination of colors, where X is
// the number of creatures with defender you control" (Produced$ Combo Any,
// kop.AfManaChoice with ManaColors == ColAny; comboAnyMana). gorge's payment
// planner cannot price it (pay.PaymentPlanChoiceColours has no colour list for
// an any-combination production), so the source is a census gap: a plain cast
// or activation only its mana pays is insufficient in the planner and gets a
// prefix search (rules potentialPrefixScript) that activates the source and
// answers its colour ask, one colour per unit. The kernel mirrors it as a gap
// kind of its own (gfGapOther, censusGapMana; never a planner unit, no gfManaVar
// view), modelled like the Lion's Eye Diamond's (mana_discardhand.go): when the
// board holds no gap the chain cannot carry (caBoard), availScript counts each
// source's amount of any-colour units, and scriptOffers searches the plays the
// sources' mana would pay: the dedicated search (caScript) when these are the
// controller's only gaps, else the scriptDFS chain with the other gaps in
// battlefield order, the source branched on its colour answers. A payment window
// (a cast's X or pip ask, a ward or unless cost) taps the source for its amount
// (windowComboTap, availCA beside availXWindow). A plain {N},{T} filter (a Signet, a
// filter land) beside it is chained by the filter search (otherComboAny,
// mana_mixgaps.go): gorge's PotentialMana prices the source unbounded, so it covers
// a first-only filter's fee (filtFirstAdmit). A gated-pip, dynamic-amount or
// restricted-source filter table, a tap-another source, a restricted or costed
// granted mana ability or a Diamond in the table is refused at its start
// (StallComboAnyMix); a board with a filter and a Combo Any source the search does
// not chain stalls by name (caOffersBoard). A plain granted mana ability beside it
// (Citanul Hierophants, Rishkar, Abundant Growth) is one search with the grants
// (mana_grantcomboany.go); a reflected source is a planner unit beside the gap.
//
// The ask is gorge's scriptAnswers: the per-unit colours of an N-unit allocation
// in odometer order (unit 0 the slowest digit, W U B R G), at most 64 answers; one
// unit lists its five. scriptOrderAnswers then moves the answers that name a
// colour the play's cost announces to the front, order kept inside each half.

// maxCAAnswers is gorge's cap on an N-unit allocation's answers (scriptAnswers).
//
//lockstep:const engine/rules.scriptAnswersCap
const maxCAAnswers = 64

// comboAnyMana reports whether a is an unrestricted "{T}: Add N mana in any
// combination of colors" ability (N dynamic or at least two): the Combo Any
// census gap. A restricted one keeps the wide any-colour model
// (restrWideAnyMana), one with a cost beyond the tap / once-each-turn flags its
// own lowering or refusal.
//
// The printed colours may be a finite mask (Vivi Ornitier's "Add X mana in any
// combination of {U} and/or {R}"): the answers are then the mask's colours, not
// WUBRG (caAnswerCountMask). A {0} once-each-turn ability (Vivi) carries no
// CostTap, so the activation taps nothing and the once-each-turn bit is
// perm.actUsed (CostOncePerTurn).
func comboAnyMana(a *kop.Ability) bool { return kop.ComboAnyManaAb(a) }

// caMask is the colours the Combo Any source at sl can add (a finite mask, or
// ColAny for the printed any-colour shapes).
func (s *State) caMask(k *prep, sl Slot) kop.ColorMask {
	return k.comboAnyAb(s.cardAt(sl)).ManaColors
}

// caGapLive is gapLive for a Combo Any source: a source whose activation costs
// no {T} (Vivi's {0} ability) can be used tapped or summoning-sick, and the
// ability's own Trig.Cond gate (PlayerTurn$ True) is read here per activation.
// The once-each-turn bit is CostOncePerTurn (perm.actUsed, caTap sets it).
func (s *State) caGapLive(k *prep, p uint8, sl Slot, a *kop.Ability) bool {
	pm := &s.perms[sl]
	if s.pickedForMana(pm.obj) || s.genLost(k, sl) || s.unlessCensusSkips(a) ||
		k.gen.anyCantAct && s.actForbidden(k, sl, true) {
		return false
	}
	if a.Cost.Flags&kop.CostTap != 0 && (pm.flags&pfTapped != 0 || s.activationSick(k, sl)) {
		return false
	}
	if a.Trig.Cond != 0 && !s.condEval(k, a.Trig.Cond, evalCtx{src: pm.obj, srcSl: sl, ctrl: pm.ctrl}) {
		return false
	}
	idx := k.gen.gapOther(s.cardAt(sl))
	return a.Cost.Flags&kop.CostOncePerTurn == 0 || idx < 0 || idx >= 8 || pm.actUsed&(1<<uint(idx)) == 0
}

// caFiniteYields reports whether the source at sl is a finite-mask Combo
// source (Vivi Ornitier's {U}/{R}) adding mana now: one of the sources gorge's
// window flag counts (untappedManaSourceFor, windowSourceYields). A Combo Any
// source's open bound is read before that flag (caPotentialOpen); a finite
// one's unbounded faces are the potential walk's own fold, which the flag
// gates.
func (s *State) caFiniteYields(k *prep, p uint8, sl Slot) bool {
	c := s.cardAt(sl)
	if !k.isComboAny(c) || k.isReflAmt(c) {
		return false
	}
	a := k.comboAnyAb(c)
	return a.ManaColors != kop.ColAny && s.caGapLive(k, p, sl, a) && s.caAmount(k, sl) > 0
}

// isComboAny reports whether card c's census gap (gfGapOther) is its Combo Any
// ability.
func (k *prep) isComboAny(c CardID) bool {
	return k.gen.anyComboAny && k.gen.isComboAnyCard(c, &k.t.Cards[c])
}

// comboAnyAb is the Combo Any ability of card c (isComboAny).
func (k *prep) comboAnyAb(c CardID) *kop.Ability {
	return &k.t.Cards[c].Abilities[k.gen.gapOther(c)]
}

// caAmount is the mana the Combo Any source at sl adds now.
func (s *State) caAmount(k *prep, sl Slot) int {
	a := k.comboAnyAb(s.cardAt(sl))
	if a.AFlags&kop.AfManaDynamic != 0 {
		return min(max(s.dynManaN(k, a, sl), 0), maxDynMana)
	}
	return max(int(a.ManaN), 1)
}

// caGaps: p's Combo Any sources that can be activated now and add mana.
func (s *State) caGaps(k *prep, p uint8) slotMask { return s.caGapsF(k, p, false) }

// caGapsF is caGaps with the forage sources (mana_forage.go): left out (the
// windows' and the other searches' view) unless forage, then those whose forage is
// payable are listed (the Combo Any search's).
func (s *State) caGapsF(k *prep, p uint8, forage bool) slotMask {
	if !k.gen.anyComboAny {
		return slotMask{}
	}
	var m slotMask
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		if !(k.isComboAny(c) || k.isReflAmt(c)) {
			continue
		}
		if !s.caGapLive(k, p, sl, k.comboAnyAb(c)) || s.caAmount(k, sl) == 0 ||
			k.forageCA(c) && !(forage && s.foragePayable(k, p, sl)) {
			continue
		}
		m = m.with(sl)
	}
	return m
}

// caBoard reports whether p controls a live Combo Any source and every census
// gap beside it is of a kind the prefix search models together with it: an
// each-colour, restricted, any-colour restricted, Taps-deferred or counter-cost
// source (the scriptDFS chain). A filter, a tap-another source, a granted mana
// ability or a Lion's Eye Diamond beside it is refused at the table's start
// (StallComboAnyMix).
func (s *State) caBoard(k *prep, p uint8) bool {
	if k.gen == nil || !k.gen.anyComboAny || !s.caGapsF(k, p, true).any() {
		return false
	}
	if k.gen.anyDiscHand && s.dhGaps(k, p).any() || s.scriptFilt(k, p).any() {
		return false
	}
	return !(k.tapMana && s.scriptTapX(k, p).any() || s.liveGrantGap(k, p) || s.grantGapLive(k, p))
}

// caPure reports whether the Combo Any sources are p's only census gaps: the
// dedicated search (caScript) then runs; beside another modelled gap kind the
// scriptDFS chain does.
func (s *State) caPure(k *prep, p uint8) bool {
	return !s.scriptGaps(k, p).or(s.scriptRestr(k, p)).or(s.scriptAnyR(k, p)).or(s.scriptTapsDefer(k, p)).or(s.scriptCtr(k, p)).any()
}

// caSearch reports whether gorge's prefix search over p's census gaps finds a
// script for play a (the gaps' chain: scriptDFS beside another gap kind).
func (s *State) caSearch(k *prep, a Action, p uint8, restr, anyR, tapsDef, ca slotMask) bool {
	if s.caPure(k, p) {
		return s.caScript(k, a, p, false)
	}
	return !s.scriptDeadEndAny(k, a, p, restr, anyR, tapsDef, s.scriptCtr(k, p), ca)
}

// caSpendable is the any-colour restricted gaps (Gwenna, Eyes of Gaea) gorge's
// search keeps for play a: a plain cast or an activation of the printed cost
// drops each source whose mana the payment cannot spend (potential_script.go
// scriptSpendableGaps, CR 106.6), so the search does not spend nodes on it nor
// tap it. scriptDeadEndAny and scriptPrefixAny apply it to every anyR they are
// handed; any other play keeps every gap.
func (s *State) caSpendable(k *prep, a Action, p uint8, anyR slotMask) slotMask {
	plain := a.Kind() == AkActivate || a.Kind() == AkCast && a.Mode() == 0 && a.Target() == 0
	if !plain {
		return anyR
	}
	for b := anyR; b.any(); b = b.rest() {
		if sl := b.first(); !s.wideAdmits(k, a, p, slotBit(sl)) {
			anyR = anyR.andNot(slotBit(sl))
		}
	}
	return anyR
}

// caPrefix plays the script gorge's search finds for play a when the plain walk
// does not list it.
func (s *State) caPrefix(k *prep, a Action, p uint8) {
	if s.caPure(k, p) {
		var d Decision
		s.plainOptions(k, &d, 0)
		if !hasOption(&d, a) {
			s.caScript(k, a, p, true)
		}
		return
	}
	s.scriptPrefixAny(k, a, p, s.scriptRestr(k, p), s.scriptAnyR(k, p), s.scriptTapsDefer(k, p), s.scriptCtr(k, p), s.caGaps(k, p))
}

// caGapAnswers is the colour ask's answer count of the census gap at sl in the
// chain: an AnyDifferent source's distinct-colour answers (not the Combo Any
// amount caAmount reads: mana_distinctcolours.go), a reflected Amount$ source's
// colour ask (mana_reflamt.go), else the Combo Any allocation's answers.
func (s *State) caGapAnswers(k *prep, sl Slot) uint8 {
	switch {
	case s.anyDifferentAt(k, sl):
		return uint8(anyDifferentAnswers(max(int(s.anyDifferentAbilityAt(k, sl).ManaN), 1)))
	case k.isReflAmt(s.cardAt(sl)):
		return s.reflAmtAnswers(k, sl)
	}
	return uint8(caAnswerCountMask(s.caAmount(k, sl), s.caMask(k, sl)))
}

// caChainTap taps Combo Any source sl for the scriptDFS chain's answer c of play
// a: the c'th answer gorge's search tries (caRank). A reflected Amount$
// source's answers are its reflected candidates in gorge's order instead, the
// whole evaluated amount in the answer's colour (mana_reflamt.go), the same
// ask caGapAnswers counts.
func (s *State) caChainTap(k *prep, p uint8, a Action, sl Slot, c int) {
	if k.isReflAmt(s.cardAt(sl)) {
		s.reflAmtChainTap(k, p, a, sl, c)
		return
	}
	need := s.playNeed(k, a)
	s.caActivate(k, p, sl, caRankMask(s.caAmount(k, sl), s.caMask(k, sl), &need, c))
}

// availCA adds each live Combo Any source's units: its amount of any colour.
// A reflected Amount$ source's window availability is gorge's bound read:
// {C} alone (mana_reflamt.go reflAmtPotential), so a coloured pip the pool
// cannot cover keeps the window unafforded, as gorge's potential fold does.
func (s *State) availCA(k *prep, p uint8, a *avail) {
	for b := s.caGaps(k, p); b.any(); b = b.rest() {
		sl := b.first()
		if s.pickCommittedSrc(k, sl) {
			continue // the pick taps it; removeSrc cannot take these units out (cast_xpicked.go)
		}
		if k.isReflAmt(s.cardAt(sl)) {
			a.addUnits(uint8(kop.ColC), uint8(s.caAmount(k, sl)))
			continue
		}
		a.addUnits(uint8(s.caMask(k, sl)), uint8(s.caAmount(k, sl)))
	}
}

// removeCA takes out of a the units availCA added for the Combo Any (or
// reflected Amount$) source at sl: a waterbend or convoke pick taps it for
// the payment, so the X window cannot spend the same tap twice
// (CR 601.2g-h). availXWindow counts these sources only on its plain walk
// (availX&3 == 0), so the same gate here removes exactly what it added.
// takeWide, not remove: availCA added group units only, no one[] entry beside
// them, and removeWide's dropOneColour would delete an unrelated manaOne
// source's entry riding the same group at the same amount (mana.go
// addOneColour), so oneColourPays would lose a source and refuse X values
// gorge offers.
func (s *State) removeCA(k *prep, a *avail, sl Slot) {
	if k.gen == nil || !k.gen.anyComboAny || s.g.availX&3 != 0 {
		return
	}
	if !s.caGaps(k, s.perms[sl].ctrl).has(sl) {
		return
	}
	n := uint8(s.caAmount(k, sl))
	if k.isReflAmt(s.cardAt(sl)) {
		a.takeWide(uint8(kop.ColC), uint16(n))
		return
	}
	a.takeWide(uint8(s.caMask(k, sl)), uint16(n))
}

// caPotentialOpen reports whether gorge's potential bound is open in every
// colour through a Combo Any source of p's: a live one whose printed mask is
// any colour. gorge's potential walk (rules potential.go addPotentialMana,
// producedOpenSet) prices a "in any combination of colors" production
// unbounded in every colour, a dynamic amount indeterminate there, so even a
// source that can make none now keeps the bound open. A trigger's own mana
// cost then looks payable beside the decline (CR 118.12): the payer elects
// pay, the window taps the source (windowGap, windowComboTap), and a charge it
// cannot fund fails (CR 118.3) and the payer declines. A finite mask (Vivi
// Ornitier's "Combo U R") is no such open: producedOpen reads it closed and
// gorge prices it unbounded in its printed faces alone
// (addIndeterminatePotentialMana, CR 107.4a), which the potential walk's own
// fold does (legal.go censusDynamicAb, addAnyDifferentOrOpen).
//
//go:noinline
func (s *State) caPotentialOpen(k *prep, p uint8) bool {
	return s.caLiveWithin(k, p, kop.ColAny)
}

// caWalkOpen reports whether p controls any live Combo source, whose mana the
// priced walk cannot count: the grant search then walks every play open and
// keeps the ones its prefix pays (caOpenGrant).
//
//go:noinline
func (s *State) caWalkOpen(k *prep, p uint8) bool {
	return s.caLiveWithin(k, p, 0)
}

// caLiveWithin reports whether p controls a live Combo source whose printed
// mask holds every colour of need.
func (s *State) caLiveWithin(k *prep, p uint8, need kop.ColorMask) bool {
	if k.gen == nil || !k.gen.anyComboAny {
		return false
	}
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		if k.isComboAny(c) && s.caMask(k, sl)&need == need && s.caGapLive(k, p, sl, k.comboAnyAb(c)) && !(k.forageCA(c) && !s.foragePayable(k, p, sl)) {
			// a reflected Amount$ source beside it is no whole-pool open — its
			// fold is {C} alone (mana_reflamt.go reflAmtPotential) — so it
			// stays out and the affords check below reads it
			return true
		}
	}
	return false
}

// caAnswerCount is the answers the colour ask of n units poses (scriptAnswers):
// a single unit's five, else the first 64 of the 5^n per-unit combinations.
func caAnswerCount(n int) int { return caAnswerCountMask(n, kop.ColAny) }

// caAnswerCountMask is caAnswerCount over the ability's printed colour mask:
// each unit's digit is one of the mask's colours (WUBRG order), so Vivi
// Ornitier's {U}/{R} units pose two answers each, not five.
func caAnswerCountMask(n int, mask kop.ColorMask) int {
	base := bits.OnesCount8(uint8(mask))
	if base == 0 {
		return 0
	}
	total := 1
	for u := 0; u < n; u++ {
		total *= base
		if total >= maxCAAnswers {
			return maxCAAnswers
		}
	}
	return total
}

// caDigit is the colour (W U B R G = 0..4) answer i gives unit u of n: i read in
// base five, unit 0 the most significant digit.
func caDigit(i, n, u int) int { return caDigitMask(i, n, u, kop.ColAny) }

// caDigitMask is caDigit over the printed colour mask: the digit indexes the
// mask's colours in WUBRG order.
func caDigitMask(i, n, u int, mask kop.ColorMask) int {
	base := bits.OnesCount8(uint8(mask))
	if base == 0 {
		return 0
	}
	for q := n - 1 - u; q > 0 && i > 0; q-- {
		i /= base
	}
	d := i % base
	for c := 0; c < kop.NumManaKinds; c++ {
		if mask&(1<<uint(c)) != 0 {
			if d == 0 {
				return c
			}
			d--
		}
	}
	return 0
}

// caNames reports whether odometer answer i of n units picks a colour need names.
func caNames(i, n int, need *[kop.NumManaKinds]bool) bool {
	return caNamesMask(i, n, need, kop.ColAny)
}

// caNamesMask is caNames over the printed colour mask.
func caNamesMask(i, n int, need *[kop.NumManaKinds]bool, mask kop.ColorMask) bool {
	for u := 0; u < n; u++ {
		if need[caDigitMask(i, n, u, mask)] {
			return true
		}
	}
	return false
}

// caRank is the odometer index of the r'th answer gorge's search tries
// (scriptOrderAnswers): the answers naming a colour the play announces first,
// then the rest, each half in odometer order; odometer order when the play names
// no colour.
func caRank(n int, need *[kop.NumManaKinds]bool, r int) int {
	return caRankMask(n, kop.ColAny, need, r)
}

// caRankMask is caRank over the printed colour mask.
func caRankMask(n int, mask kop.ColorMask, need *[kop.NumManaKinds]bool, r int) int {
	if !(need[0] || need[1] || need[2] || need[3] || need[4]) {
		return r
	}
	total := caAnswerCountMask(n, mask)
	for _, first := range [2]bool{true, false} {
		for i := 0; i < total; i++ {
			if caNamesMask(i, n, need, mask) == first {
				if r == 0 {
					return i
				}
				r--
			}
		}
	}
	return r
}

// caActivate activates the Combo Any source at sl with odometer answer ans:
// the tap (only when the cost asks for one), then one unit of each answered
// colour into p's pool.
func (s *State) caActivate(k *prep, p uint8, sl Slot, ans int) {
	n := s.caAmount(k, sl)
	s.caTap(k, sl)
	s.caAdd(k, p, sl, ans, n)
}

// caTapsSource reports whether the Combo source at sl's activation cost holds
// {T} (Vivi Ornitier's {0} ability does not).
func (s *State) caTapsSource(k *prep, sl Slot) bool {
	return k.comboAnyAb(s.cardAt(sl)).Cost.Flags&kop.CostTap != 0
}

// caTap pays Combo Any source sl's activation: the {T} when the cost has one,
// with its Taps notification, and the once-each-turn record.
func (s *State) caTap(k *prep, sl Slot) {
	s.caPayCost(k, sl)
	if s.caTapsSource(k, sl) {
		s.manaTapped(k, sl)
	}
}

// caPayCost is caTap without the Taps notification (the grant search's next
// stacks it): the {T} when the cost has one, and the once-each-turn record
// (Vivi's ActivationLimit$ 1, perm.actUsed).
func (s *State) caPayCost(k *prep, sl Slot) {
	a := k.comboAnyAb(s.cardAt(sl))
	if a.Cost.Flags&kop.CostTap != 0 {
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
	}
	if a.Cost.Flags&kop.CostOncePerTurn != 0 {
		if idx := k.gen.gapOther(s.cardAt(sl)); idx >= 0 && idx < 8 {
			s.perms[sl].actUsed |= 1 << uint(idx)
		}
	}
}

// caAdd adds the n units answer ans names to p's pool, one of the ability's
// printed colour mask per unit.
func (s *State) caAdd(k *prep, p uint8, sl Slot, ans, n int) {
	mask := s.caMask(k, sl)
	var got [kop.NumManaKinds]int
	for u := 0; u < n; u++ {
		got[caDigitMask(ans, n, u, mask)]++
	}
	for col, c := range got {
		if c > 0 {
			s.poolAdd(p, col, c)
			s.artTag(k, p, sl, col)
		}
	}
	s.tapExtraStall(k, sl)
}

type caStep struct {
	sl  Slot
	ans uint8 // the odometer answer (below maxCAAnswers)
	fo  uint8 // a forage source (mana_forage.go): the forage option answered
	sib bool  // a forage source's plain ability, not its forage one
}

// caSearch is gorge's prefix search over p's Combo Any sources
// (scriptSearch.visit, gapsOnly with no funding): at each payer priority node it
// asks whether the play is offered or the planner pays it from the pool, else
// activates each remaining source in battlefield order, branching on its colour
// answers.
type caSearch struct {
	a     Action
	p     uint8
	need  [kop.NumManaKinds]bool
	nodes int
	path  [MaxPerm]caStep
	np    int
	// seen: the priority nodes visited, by gorge's scriptStateKey: a node whose
	// pool and tapped set were seen is dead (scriptSearch.visit), so the (W,G)
	// and (G,W) answers of one source share a subtree and the budget reaches
	// the all-one-colour line of two sources.
	nseen int
	seen  [potentialScriptNodes]filtKey
	// own: the object whose {T} the searched activation costs (mana_forage.go ownTapObj)
	own Obj
	// stall: the first stall a failed line's fork latched (stallOf), StallNone
	// none; caScript declares it when the whole search fails
	stall uint8
}

// visit is scriptSearch.visit at payer priority node t (root: no source used yet,
// so the play is not priced there).
func (cs *caSearch) visit(k *prep, t *State, d *Decision, root bool) bool {
	if cs.nodes >= potentialScriptNodes {
		return false
	}
	cs.nodes++
	if !root {
		t.restrOptions(k, d, cs.p)
		if hasOption(d, cs.a) {
			return true
		}
	}
	key := t.scriptKey(k, cs.p, slotMask{})
	if t.playInHand(cs.p, cs.a) {
		key.hh = 1
	}
	for i := 0; i < cs.nseen; i++ {
		if cs.seen[i] == key {
			return false
		}
	}
	if cs.nseen < len(cs.seen) {
		cs.seen[cs.nseen] = key
		cs.nseen++
	}
	var order [MaxPerm]Slot
	n := t.battlefieldOrder(cs.p, t.caGapsF(k, cs.p, true), order[:], 0)
	for i := 0; i < n; i++ {
		if cs.nodes >= potentialScriptNodes {
			return false
		}
		if k.forageCA(t.cardAt(order[i])) {
			if cs.forageSource(k, t, d, order[i]) {
				return true
			}
			continue
		}
		if k.isReflAmt(t.cardAt(order[i])) {
			// a reflected Amount$ source: the ask is the colour candidates
			// (mana_reflamt.go), one node only when more than one candidate
			// names an ask — a single candidate resolves with none (gorge's
			// effManaReflected) — and the whole evaluated amount floats in the
			// pick's colour. A source with no candidate makes no mana (gorge's
			// scriptSpendableGaps, CR 106.6): no branch, the search goes on.
			cols := t.reflAmtCols(k, order[i])
			if cols == 0 {
				continue
			}
			if cs.nodes >= potentialScriptNodes {
				return false
			}
			if bits.OnesCount8(cols) > 1 {
				cs.nodes++ // the colour ask's node
			}
			var ans [6]uint8
			for _, col := range t.reflAmtOrder(k, order[i], cs.a, ans[:]) {
				c := t.fork()
				c.reflAmtDo(k, cs.p, order[i], int(col))
				cs.path[cs.np] = caStep{sl: order[i], ans: col}
				cs.np++
				found := cs.visit(k, c, d, false)
				if !found {
					cs.stallOf(c) // the line failed; the fork may have latched a stall
				}
				t.unfork()
				if found {
					return true
				}
				cs.np--
			}
			continue
		}
		cs.nodes++ // the colour ask's node
		mask := t.caMask(k, order[i])
		units := t.caAmount(k, order[i])
		for r, na := 0, caAnswerCountMask(units, mask); r < na; r++ {
			ans := caRankMask(units, mask, &cs.need, r)
			c := t.fork()
			c.caActivate(k, cs.p, order[i], ans)
			cs.path[cs.np] = caStep{sl: order[i], ans: uint8(ans)}
			cs.np++
			found := cs.visit(k, c, d, false)
			if !found {
				cs.stallOf(c) // the line failed; the fork may have latched a stall
			}
			t.unfork()
			if found {
				return true
			}
			cs.np--
		}
	}
	return false
}

// caScript reports whether gorge's prefix search over p's Combo Any sources
// finds a script for play a; with apply it plays the script on s (the play's
// prefix).
func (s *State) caScript(k *prep, a Action, p uint8, apply bool) bool {
	cs := caSearch{a: a, p: p, need: s.playNeed(k, a), own: s.ownTapObj(k, a)}
	var d Decision
	if !cs.visit(k, s, &d, true) {
		if cs.stall != 0 {
			// every line failed and one ran into a rule gap the kernel stalls
			// by name: declare it, not "no payment" (tapXScript's stall; a
			// found path is not this, its apply replays it on s)
			s.capFull(cs.stall)
		}
		return false
	}
	if apply {
		for _, st := range cs.path[:cs.np] {
			if k.forageCA(s.cardAt(st.sl)) {
				s.forageReplay(k, p, st) // (mana_forage.go)
				continue
			}
			if k.isReflAmt(s.cardAt(st.sl)) {
				s.reflAmtDo(k, p, st.sl, int(st.ans)) // the reflected Amount$ source: ans is the colour (mana_reflamt.go)
				continue
			}
			s.caActivate(k, p, st.sl, int(st.ans))
		}
	}
	return true
}

// stallOf records the stall a failed line's fork latched (capFull) while the
// line ran to it, the first one, so caScript declares it on the real state
// instead of dropping the play as unpayable: the fork cannot go on (its next
// stepGen ends the game), but the play could have been payable along it.
func (cs *caSearch) stallOf(c *State) {
	if cs.stall == 0 {
		cs.stall = c.g.stalled
	}
}

// windowComboTap activates Combo Any source sl in a payment window: the tap,
// then each unit's colour as the ask is answered (payexec.WindowChoices, the
// wide source's windowWideTap): the clamp's kept colours listed, each mana a
// colour the cost still owes, else the lowest listed, the owed amount falling
// by what each mana covers; a window whose ask carries no cost (noOwed) owes
// nothing, so every mana is the lowest listed.
func (s *State) windowComboTap(k *prep, p uint8, sl Slot, cost, useful *kop.ManaCost, clamp, noOwed bool) {
	if s.windowFlatCombo(k, p, sl) {
		col := s.windowFlatCol(k, p, sl, cost, useful, clamp, noOwed)
		s.caTap(k, sl)
		s.caFloat(k, p, sl, col) // the flattened ability option (mana_windowcomboflat.go)
		return
	}
	n := s.caAmount(k, sl)
	s.caTap(k, sl)
	mask := uint8(s.caMask(k, sl))
	if clamp {
		if want, decided := s.windowUseful(k, p, useful); decided && want&mask != 0 {
			mask &= want
		}
	}
	var owed [len(cost.Pips)]uint8
	if !noOwed {
		owed = cost.Pips
		for c := range owed {
			owed[c] -= min(owed[c], s.pl[p].pool[c])
		}
	}
	var got [5]int
	for i := 0; i < n; i++ {
		col := bits.TrailingZeros8(mask)
		for c := 0; c < 5; c++ {
			if mask&(1<<c) != 0 && owed[c] > 0 {
				col = c
				owed[c]--
				break
			}
		}
		got[col]++
	}
	for col, c := range got {
		if c > 0 {
			s.poolAdd(p, col, c)
			s.artTag(k, p, sl, col)
		}
	}
	s.tapExtraStall(k, sl)
}

// StallComboAnyMix: a table holding an unrestricted Combo Any source
// (comboAnyMana) beside a mana source of a census-gap kind the kernel does not
// search together with it: a filter, a tap-another source, a granted mana ability
// or a Lion's Eye Diamond. gorge's one prefix search activates every census gap
// together (potential_script.go scriptSearch.visit); the kernel chains the Combo
// Any source with the each-colour, restricted, any-colour restricted,
// Taps-deferred and counter-cost sources (caBoard), so the others are refused at
// the game's start. The check is table-wide: a Combo Any source in one deck and a
// filter in the other is refused too.
const StallComboAnyMix = 183

func init() { stallNames[StallComboAnyMix] = "comboanymix" }

// prepComboAnyMix sets genPrep.caMix when t holds a Combo Any source and a source
// of a gap kind caBoard does not chain: StallComboAnyMix.
func prepComboAnyMix(t *kop.Table, g *genPrep, p *prep) {
	if !g.anyComboAny {
		return
	}
	g.caMix = g.anyFilt && caFiltRefused(g) || p.tapMana || len(g.grantPC) != 0 && caGrantRefused(g) || g.anyDiscHand || g.anyRefl && DebugComboAnyReflRefused
	g.caMix = g.caMix || forageMixTable(g, p)
	for i := range t.Cards {
		g.caMix = g.caMix || caMixGap(t, g, p, i)
	}
	p.caMixForage = g.caMix && forageCAOnly(t, g)
}

// caMixGap reports whether card i carries a census-gap kind caBoard does not
// chain with a Combo Any source.
func caMixGap(t *kop.Table, g *genPrep, p *prep, i int) bool {
	c := CardID(i)
	if g.has(c, gfDiscHand) || p.tapXAb[i] >= 0 || forageMixGap(t, g, p, i) {
		return true // a Lion's Eye Diamond, a tap-another source (mana_discardhand.go, mana_tapanother.go): a plain filter is chained (caFiltRefused)
	}
	return caGrantRefused(g) && abilityOn(func(a *kop.Ability) bool { return a.Kind == kop.AbMana && a.AFlags&kop.AfLatent != 0 })(&t.Cards[i], i) // a granted mana ability the grant search does not chain (caGrantRefused)
}

// caOffersBoard reports whether p's live Combo Any source is one scriptOffers
// models: alone (ca, caBoard) or beside a live filter (caFiltBoard). A live
// Combo Any source beside a live filter on a board neither search chains (an
// each-colour source among the gaps, say) stalls the game by name
// (StallComboAnyMix): gorge's one prefix search activates all of them, so the
// kernel declares the gap rather than offer plays on half a census.
func (s *State) caOffersBoard(k *prep, p uint8, ca bool) bool {
	if ca {
		return true
	}
	if k.gen == nil || !k.gen.anyComboAny {
		return false
	}
	if s.caFiltBoard(k, p) {
		return true
	}
	if s.caGaps(k, p).any() && s.scriptFilt(k, p).any() {
		s.capFull(StallComboAnyMix)
	}
	return false
}

// caFiltRefused reports whether the table's filters are of a shape the filter
// search does not take a Combo Any source beside (otherComboAny, mana_mixgaps.go):
// only the plain {N},{T} filters (Signets, the filter lands) are chained. A
// first-ability, gated-pip or dynamic-amount filter, or an each-colour or
// restricted source beside the filters (mixBoard does not count them), stays
// refused (StallComboAnyMix). DebugComboAnyFiltRefused (mutation test only)
// refuses every filter, as before the chain.
func caFiltRefused(g *genPrep) bool {
	return DebugComboAnyFiltRefused || g.anyFiltPip || g.anyDynFilt || g.anyRestrScript
}

// DebugComboAnyFiltRefused (mutation test only): a Combo Any source beside a
// filter is refused at the table's start, as before otherComboAny.
var DebugComboAnyFiltRefused bool

// DebugComboAnyFiltNoChain (mutation test only): the filter search leaves the
// Combo Any source out of its gaps, so a Signet's fee is never funded from it.
var DebugComboAnyFiltNoChain bool

// isComboAnyCard reports whether card c's census gap is its unrestricted Combo Any
// ability (isComboAny without a prepared table).
func (g *genPrep) isComboAnyCard(c CardID, cd *kop.CardDef) bool {
	if !g.has(c, gfGapOther) {
		return false
	}
	j := g.gapOther(c)
	return j >= 0 && comboAnyMana(&cd.Abilities[j])
}

// DebugComboAnyReflRefused (mutation test only): a Combo Any source beside a
// reflected mana source (Fellwar Stone, Exotic Orchard, Plaza of Heroes) is
// refused at the table's start, as before the lift. A reflected source is a
// planner unit (its view is a function of the board, manaReflView), so the
// Combo Any chain adds its gap beside it as beside any printed source.
var DebugComboAnyReflRefused bool

// caRelaxedMax is the largest amount gorge's relaxed census enumerates for an
// any-combination production (relaxedAddProduction: one alternative per colour
// multiset, none above six units).
const caRelaxedMax = 6

// caRelaxable reports whether gorge's census is relaxable on a board whose only
// gaps are fixed-amount Combo Any sources: each is a once-only {T} ability the
// relaxed census enumerates (paymentPlanRelaxedAlternatives), so the relaxed
// proof drops every play no colour multiset of the sources' amounts pays as
// "insufficient relaxed_census" and it takes no prefix-search slot
// (potentialCensusFallback). The candidates are then the gaps walk's alone,
// not the any-mana walk's whole potential list: Sage of the Maze's two mana
// beside a Staff of Domination and four Mutagen tokens left the seventeenth
// play of the any-mana walk out of gorge's sixteen slots, though gorge's walk
// listed only the nine plays the two mana pays. A dynamic amount (Axebane
// Guardian), a reflected one, or an amount above caRelaxedMax is no such
// source and stays unrelaxable.
func (s *State) caRelaxable(k *prep, p uint8, ca bool) bool {
	if !ca || !s.caPure(k, p) || s.anyDifferentGaps(k, p).any() || s.lifeGapMask(k, p).any() {
		return false
	}
	for b := s.caGaps(k, p); b.any(); b = b.rest() {
		sl := b.first()
		if k.isReflAmt(s.cardAt(sl)) {
			return false
		}
		if a := k.comboAnyAb(s.cardAt(sl)); a.AFlags&kop.AfManaDynamic != 0 || int(a.ManaN) > caRelaxedMax {
			return false
		}
	}
	return true
}
