package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Mixed census gaps. gorge's prefix search (rules scriptSearch.visit) activates
// every source its census misses, whatever makes it a gap, depth first in
// priority-option order, and funds a filter's fee from the others only after
// the gaps. filtSearch is that search over filters; on a board that mixes a
// filter, a tap-another-permanent source and a restricted any-colour source
// (Interdimensional Web Watch beside Daily Bugle Building, or beside Supportive
// Parents), and on any filter board with a granted mana ability (Honest Work's
// {C}), the other gaps are its otherGap steps: their answers branched on in
// gorge's order, their asks charged to the node budget.

const availNoWide uint16 = 16 // the gaps walk without a wide source's mana (availAnyR)

// availNoFilt is the gaps walk with no filter activated: gorge's relaxed proof
// (paymentPlanRelaxProof) accepts any subset of the paid gaps, the empty one
// included, while the gaps walk charges every filter's fee. A fee that
// exceeds its filter's production (School of the Unseen's {2},{T}: one mana
// of any color) costs a play the other sources pay on their own.
const availNoFilt uint16 = 256

const (
	otherNone uint8 = iota
	otherAnyR
	otherTapX
	otherGrant
	otherAnswer    // filtStep.act answers the ask a step's Taps trigger posed
	otherPass      // p passes with a trigger of the path on the stack
	otherDeferred  // a deferred source tapped for its {R} (mixDeferred)
	otherTapXPlain // a second-ability tap-another source's plain first ability (tapXPlainDo)
	otherCtr       // a counter-cost source's ability (mixCtr): the counter one, or plain {T} first
	otherChosen    // a chosen-colour pair source (mana_chosenfilter.go): its plain {T} ability or the land-only pair
	otherTapsDefer // a Taps-deferred gap (Grove of the Burnwillows' rider) beside a filter: the script search's answer ans (tapsDeferApply)
	otherRider     // a bounded rider source (mixRider): ans is tapGapChoice's engine-order answer
	otherComboAny  // a Combo Any source (Axebane Guardian, mana_comboany.go): ans is the odometer answer (caActivate)
	otherLife      // a life-only mana ability (kop.GapLifeManaAb): ans is the colour it makes (lifeGapDo)
	otherLifePain  // a sole life-only pain unit (lifeOnlyPain): ans is the colour it makes (lifePainDo)
	otherEach      // a free each-colour source (Faeburrow Elder, mixEach): one of each colour among its permanents
)

// otherStep is one activation of a gap that is no filter.
type otherStep struct {
	kind        uint8
	ans         uint8 // otherAnyR: the scriptDFS answer; otherTapX, otherGrant: the colour
	n           uint8 // otherGrant: the mana it makes
	gi          int16 // otherGrant: the grantPC index; -1 the source's own ability
	cand, cand2 Slot  // otherTapX: the permanents its cost taps
	// col: otherCtr's colour (-1: the ability asks none); plain: otherCtr's
	// activation ask answered with the card's plain first {T} ability
	// (ctrAb1) rather than its counter one (ctrGap)
	col   int8
	plain bool
	// x, xset: otherCtr's storage battery X ask answer (storeGapX): gorge's
	// ask offers every X from the counters held down to 0
	x    uint8
	xset bool
}

// mixBoard: p's census gaps mix filters, restricted any-colour sources,
// tap-another-permanent sources and counter-cost sources, or a filter beside
// a deferred source (mixDeferred), a sacrifice-another one (Giant's Boulder's
// {1},{T} beside Bolg's Company) or a counter-cost one (mixCtr), and hold
// nothing else. gorge's one prefix search activates every census gap in
// battlefield order, and the filter search takes the non-filter gaps as
// otherGap steps: otherGap's sacrifice-another, deferred and counter arms.
func (s *State) mixBoard(k *prep, p uint8) bool {
	if k.gen != nil && k.gen.anyMultiR && s.wide2Board(k, p) {
		return true // any-colour restricted gaps, a second-ability wide one among them (mana_restrictedcombo.go)
	}
	if k.gen == nil || !k.gen.anyFilt && !k.tapMana || !k.gen.anyAnyR && !k.gen.anyComboAny && !k.tapMana && !k.gen.anyEachCol && !k.gen.anyDeferred && !k.ctrMana && !k.gen.anyManaRider && !k.gen.uses.has(useLifeGap) {
		return false
	}
	filt := s.scriptFilt(k, p).any()
	if k.tapMana && s.sacXLive(k, p) && !filt {
		return false // no filter gap beside: a sacrifice-another source is the tap-another search's alone (mana_tapanother.go)
	}
	n := 0
	if filt {
		n++
	}
	def := s.mixDeferred(k, p, filt)
	if def.any() {
		n++
	}
	if s.scriptAnyR(k, p).any() {
		n++
	}
	if s.scriptTapX(k, p).any() {
		n++
	}
	ctr := s.mixCtr(k, p, filt)
	if ctr.any() {
		n++
	}
	if s.mixRider(k, p, filt).any() {
		n++
	}
	if s.mixComboAny(k, p, filt).any() {
		n++
	}
	if s.mixEach(k, p, filt).any() {
		n++
	}
	if s.lifeGapMask(k, p).any() {
		n++
	}
	// A counter source beside no filter stays the counter-gap scripts' (a
	// tap-another or any-colour board): only a counted one is the filter
	// search's otherGap step.
	return n >= 2 && !s.scriptGaps(k, p).andNot(s.mixEach(k, p, filt)).or(s.scriptRestr(k, p).andNot(def)).or(s.scriptCtr(k, p).andNot(ctr)).any()
}

// mixDeferred: p's activatable deferred sources (Pyromancer's Goggles,
// mana_deferred.go) when a filter is a gap beside them (filt). gorge's census
// misses both (a paid filter, a relaxed deferred {R}), and its one prefix
// search (scriptSearch.visit) activates every census gap in battlefield
// order, so the filter search takes the deferred source as an otherGap step:
// Pyromancer's Goggles' {R} pays Three Tree Mascot's {1}, whose {U} and a
// Plains' {W} cast Negate (CR 601.2g-h, 605.3a). Without a filter the deferred
// source keeps the restricted-gap scripts (scriptRestr).
func (s *State) mixDeferred(k *prep, p uint8, filt bool) slotMask {
	if !filt || !k.gen.anyDeferred {
		return slotMask{}
	}
	return s.scriptDeferred(k, p)
}

// mixCtr: p's activatable counter-cost sources (mana_countercost.go
// scriptCtr) when a filter is a gap beside them (filt). gorge's census misses
// both (a paid filter, a counter-cost source), and its one prefix search
// activates every census gap in battlefield order, so the filter search takes
// the counter source as an otherGap step: Weather Maker's "{T}, remove two
// charge counters: Add {C}{C}" funds Omni-Cheese Pizza's {1} fee for {W} and
// pays the rest of Leonardo, Leader in Blue's {1}{W} activation (CR 605.3a,
// 601.2g-h). A counter source short of counters is no gap (scriptCtr), so a
// filter beside it stays filter-only (gapOthers drops it).
func (s *State) mixCtr(k *prep, p uint8, filt bool) slotMask {
	if !filt || !k.ctrMana {
		return slotMask{}
	}
	return s.scriptCtr(k, p)
}

// mixRider: p's activatable bounded rider sources (mana_rider.go: Peat Bog's
// "{T}, remove a depletion counter: Add {B}{B}") when a filter is a gap beside
// them (filt). gorge's planner defers the rider ability, so its census misses
// both, and its one prefix search activates every census gap in battlefield
// order: the filter search takes the rider source as an otherGap step, Peat
// Bog's {B}{B} funding Gruul Signet's {1} fee for {R}{G} and paying the rest
// of Dragon Throne of Tarkir's Equip {3} (CR 605.3a, 601.2g-h). A rider whose
// plain first ability shares its ask (riderPlainFirst: Grove of the
// Burnwillows beside Darkwater Catacombs) is a gap too; a board holding
// another Taps-deferred gap keeps the Taps-deferred search (scriptPrefixAny).
func (s *State) mixRider(k *prep, p uint8, filt bool) slotMask {
	if !filt || !k.gen.anyManaRider {
		return slotMask{}
	}
	td := s.scriptTapsDefer(k, p)
	var m slotMask
	for b := td; b.any(); b = b.rest() {
		sl := b.first()
		if _, ok := s.riderAbilityOf(k, sl); ok {
			m = m.with(sl)
		}
	}
	if td.andNot(m).any() {
		return slotMask{}
	}
	return m
}

// mixComboAny: p's live Combo Any sources (mana_comboany.go caGaps) when a
// filter is a gap beside them (filt). gorge's census misses both (an
// any-combination production, a paid filter) and its one prefix search
// activates every gap in battlefield order, funding a filter's fee from the
// others after them: the filter search takes the Combo Any source as an
// otherComboAny step, Axebane Guardian's X mana (one colour each, any
// combination) paying a Signet's {1} (CR 605.3a, 601.2g-h). A reflected
// Amount$ source is a step too, its one colour ask (reflAmtOther).
func (s *State) mixComboAny(k *prep, p uint8, filt bool) slotMask {
	if !filt || DebugComboAnyFiltNoChain || !k.gen.anyComboAny {
		return slotMask{}
	}
	return s.caGaps(k, p)
}

// mixEach: p's live free each-colour sources (Faeburrow Elder, "{T}: for each
// colour among permanents you control, add one mana of that colour") when a
// filter is a gap beside them (filt). gorge's census misses both (a
// Special production, a paid filter) and its one prefix search activates every
// gap in battlefield order (rules scriptSearch.visit), funding a filter's fee
// from the pool the others made: the filter search takes the source as an
// otherEach step, the Elder's {G}{W} paying a Signet's {1} (CR 605.3a,
// 601.2g-h). A fee-paid one (Tarnation Vista) keeps eachSearch's ask nodes
// and stays refused beside a filter (mixBoard).
func (s *State) mixEach(k *prep, p uint8, filt bool) slotMask {
	if !filt || DebugEachNoFilterChain || !k.gen.anyEachCol {
		return slotMask{}
	}
	var m slotMask
	for b := s.scriptGaps(k, p); b.any(); b = b.rest() {
		if sl := b.first(); s.eachFee(k, sl) == 0 {
			m = m.with(sl)
		}
	}
	return m
}

// DebugEachNoFilterChain (mutation test only): an each-colour source beside a
// filter is no step of the filter search again.
var DebugEachNoFilterChain bool

// caFiltBoard reports whether p's live Combo Any source is a gap of the filter
// search (mixComboAny beside a live filter on a mixBoard).
func (s *State) caFiltBoard(k *prep, p uint8) bool {
	return k.gen != nil && k.gen.anyComboAny && k.gen.anyFilt && s.scriptFilt(k, p).any() && s.mixComboAny(k, p, true).any() && s.mixBoard(k, p)
}

// comboAnyOther branches Combo Any gap sl's colour ask at a priority node (the
// caSearch's node): one node, then the allocation's answers in gorge's order,
// the colours the play announces first (caRank).
func (fs *filtSearch) comboAnyOther(t *State, k *prep, sl Slot, try func(otherStep) bool) bool {
	if k.isReflAmt(t.cardAt(sl)) {
		return fs.reflAmtOther(t, k, sl, try) // a reflected Amount$ source names one colour for all its mana (mana_reflamt.go)
	}
	if !fs.tick() {
		return false
	}
	units := t.caAmount(k, sl)
	for r, na := 0, caAnswerCount(units); r < na; r++ {
		if try(otherStep{kind: otherComboAny, ans: uint8(caRank(units, &fs.need, r))}) {
			return true
		}
	}
	return false
}

// riderOther branches rider gap sl's answers at a priority node (otherGap's
// last arm; false for a slot that is no mixRider gap): its colour ask, when it
// has one, is one node, its answers in scriptDFS's order (the play's announced
// colours first, scriptEngineAnswer). A riderPlainFirst source poses one
// production ask: its plain mana (answer 0), then the rider's colours,
// riderPlainGap's order; the step keeps the action it plays under.
func (fs *filtSearch) riderOther(k *prep, t *State, sl Slot, funded int, try func(otherStep) bool) bool {
	if !t.mixRider(k, fs.p, true).has(sl) {
		return false
	}
	if t.riderPlainFirst(k, sl) {
		if !fs.tick() { // the production ask
			return false
		}
		for c := uint8(0); c < t.deferGapAnswers(k, sl); c++ {
			if fs.try(k, t, filtStep{sl: sl, gap: true, other: otherStep{kind: otherRider, ans: c}, act: fs.a}, funded, true) {
				return true
			}
		}
		return false
	}
	td := slotMask{}.with(sl)
	n := t.deferGapAnswers(k, sl)
	if n > 1 && !fs.tick() {
		return false
	}
	for c := 0; c < int(n); c++ {
		if try(otherStep{kind: otherRider, ans: uint8(t.scriptEngineAnswer(k, fs.a, sl, slotMask{}, td, c))}) {
			return true
		}
	}
	return false
}

// otherGaps: the gaps a filter search on p's board activates beside its
// filters. A play's own granted sources are the grant search's (grantCap /
// grantPrefix) too, but gorge runs one prefix search over the filters and the
// grants in battlefield order; an activation's filter search counts them (its
// noGrant pricing still keeps them out of the availability check, so only the
// search's own activation steps add their mana).
func (s *State) otherGaps(k *prep, p uint8, a Action) slotMask {
	var m slotMask
	if s.mixBoard(k, p) {
		filt := s.scriptFilt(k, p).any()
		m = s.scriptAnyR(k, p).or(s.scriptTapX(k, p)).or(s.mixDeferred(k, p, true)).or(s.mixCtr(k, p, filt)).or(s.mixRider(k, p, filt)).or(s.mixComboAny(k, p, filt)).or(s.mixEach(k, p, filt)).or(s.lifeGapMask(k, p))
	}
	if len(k.gen.grantPC) != 0 && s.grantsLive(k) {
		for b := s.ctrlM[SeatIx(p)].and(s.occ); b.any(); b = b.rest() {
			if sl := b.first(); s.grantGapUsable(k, sl) {
				m = m.with(sl)
			}
		}
	}
	m = m.or(s.chosenPairGaps(k, p)) // (a filter board only: mana_chosenfilter.go)
	if s.scriptFilt(k, p).any() {
		m = m.or(s.scriptTapsDefer(k, p)) // gorge's one prefix search activates a Taps-deferred gap beside the filters too (tapsDeferGap)
	}
	return m.andNot(s.scriptFilt(k, p))
}

// anyRSteps drives one single-colour anyR gap's six scriptDFS answers in
// gorge's order: answer 0 (the first printed mana ability) before the colour
// ask's node -- gorge's "choose a mana ability" ask has no ManaSymbol, so
// scriptOrderAnswers leaves it alone -- then the colour ask with the colours
// the play announces first and the rest in engine order. tick charges the
// colour ask's node; try submits one answer and reports whether the search
// below it found the play.
func anyRSteps(need [kop.NumManaKinds]bool, try func(uint8) bool, tick func() bool) bool {
	if try(0) {
		return true
	}
	if !tick() {
		return false
	}
	for c := uint8(1); c <= 5; c++ {
		if need[c-1] && try(c) {
			return true
		}
	}
	for c := uint8(1); c <= 5; c++ {
		if !need[c-1] && try(c) {
			return true
		}
	}
	return false
}

// otherOwnAsk branches the gaps whose asks their own helpers pose: a
// chosen-colour pair, a deferred {R}, a counter-cost source.
// handled: sl is one of them, found what its branch found.
func (fs *filtSearch) otherOwnAsk(k *prep, t *State, sl Slot, funded int, try func(otherStep) bool) (found, handled bool) {
	switch {
	case t.chosenPairGapSrc(k, sl):
		return fs.chosenPairGap(t, sl, try), true
	case k.gen.anyDeferred && k.gen.has(t.cardAt(sl), gfDeferred):
		if t.perms[sl].flags&pfTapped != 0 || !t.deferLifeSurvives(k, fs.p, sl) {
			return false, true
		}
		if k.deferIdent(t.cardAt(sl)) {
			return fs.deferIdentOther(k, t, sl, try), true // the identity's colour ask (mana_deferident.go)
		}
		// a single fixed {R} ability: no ask node (deferBranch)
		return try(otherStep{kind: otherDeferred, col: -1}), true
	case t.scriptCtr(k, fs.p).has(sl):
		return fs.ctrOther(k, t, sl, try), true
	case t.isLifeGapAt(k, fs.p, sl):
		return fs.lifeGap(k, t, sl, funded, try), true
	}
	return false, false
}

// tapXGap branches on the tap-another or sacrifice-another gap sl's answers at
// a priority node: the ability ask of a second-ability source, the permanent
// to tap or sacrifice, then the colour of an any-colour one.
func (fs *filtSearch) tapXGap(k *prep, t *State, sl Slot, try func(otherStep) bool) bool {
	p := fs.p
	if k.tapXSecond(t.cardAt(sl)) && t.canTapForMana(k, sl) {
		// its ability ask (tapXAbilityAsk): the plain first ability, then
		// the tap-another one. A tapped source has only the second: its {T}
		// is already spent, and tapXPlainDo would tap it again for a free
		// unit (Relic of Legends tapped, its legendary-creature ability the
		// only one left).
		if !fs.tick() {
			return false
		}
		if mc := k.cards[t.cardAt(sl)].manaCol; mc&(mc-1) != 0 {
			// the plain ability is an any-colour one: its colour ask behind
			// the ability ask, the play's announced ones first, as the
			// tap-another search's colour asks order them (gorge
			// scriptOrderAnswers; the payment window's answer is a colour
			// the play still owes, not W)
			if !fs.tick() {
				return false
			}
			var order [5]uint8
			no := 0
			for col := uint8(0); col < 5; col++ {
				if fs.need[col] {
					order[no] = col
					no++
				}
			}
			for col := uint8(0); col < 5; col++ {
				if !fs.need[col] {
					order[no] = col
					no++
				}
			}
			for i := 0; i < 5; i++ {
				if try(otherStep{kind: otherTapXPlain, ans: order[i]}) {
					return true
				}
			}
			// no colour of the plain ability paid: the ask's other answer,
			// the tap-another one, is below
		} else if try(otherStep{kind: otherTapXPlain}) {
			return true
		}
	}
	var cs [MaxPerm]Slot
	nc := t.tapXCands(k, p, sl, &cs)
	colours := func(a, b Slot) bool {
		if fc, ok := kop.TapXFixedColour(k.tapXAbOf(t.cardAt(sl))); ok {
			// a fixed production asks no colour
			return try(otherStep{kind: otherTapX, ans: uint8(fc), cand: a, cand2: b})
		}
		if !fs.tick() { // the colour ask
			return false
		}
		// gorge's scriptOrderAnswers on the same ask: a colour the play
		// announces first, the rest in engine order
		var order [5]uint8
		no := 0
		for col := uint8(0); col < 5; col++ {
			if fs.need[col] {
				order[no] = col
				no++
			}
		}
		for col := uint8(0); col < 5; col++ {
			if !fs.need[col] {
				order[no] = col
				no++
			}
		}
		for i := 0; i < 5; i++ {
			if try(otherStep{kind: otherTapX, ans: order[i], cand: a, cand2: b}) {
				return true
			}
		}
		return false
	}
	if k.exileGOf(t.cardAt(sl)) {
		return fs.exileGGap(nc, &cs, colours) // the exile pick, then the colour (mana_exilegrave.go)
	}
	if k.sacXOf(t.cardAt(sl)) {
		// A sacrifice-another source asks no colour: its whole production
		// floats at once (sacXDo, gorge's commit order); only the "which
		// permanent" ask is a node, with two or more candidates, as the
		// tap-another search's is (mana_tapanother.go).
		if nc > 1 && !fs.tick() {
			return false
		}
		for i := 0; i < nc; i++ {
			if tapXOwnAnswer(fs.own, cs[i], NoSlot) {
				// The play's own {T} source as the sacrifice: gorge
				// submits that answer and the node it reaches ends the
				// line in leftOwnZone (potential_script.go, CR 602.2b),
				// one node spent, ask or no ask -- the plain tap-another
				// cost's own answers are skipped before their ask
				// branches (tapsOwnSource) and cost none there. The
				// search keeps its budget charged without the fork.
				if !fs.tick() {
					return false
				}
				continue
			}
			if kop.SacXAnyColour(k.tapXAbOf(t.cardAt(sl))) {
				if colours(cs[i], NoSlot) { // an any-colour one asks its colour after the sacrifice
					return true
				}
				continue
			}
			if try(otherStep{kind: otherTapX, cand: cs[i]}) { // otherDo -> tapXDo -> sacXDo
				return true
			}
		}
		return false
	}
	if k.tapXAbOf(t.cardAt(sl)).Cost.N == 2 {
		// two permanents: no ask for exactly two, else one "choose 2" whose
		// answers scriptAnswers enumerates as every pair of its candidates
		// in lexicographic order, 64 at most (tapXSearch.visit)
		if nc > 2 && !fs.tick() {
			return false
		}
		npair := tapXPairLimit(nc)
		for pi := 0; pi < npair; pi++ {
			i, j := tapXPairAt(nc, pi)
			a, b := cs[i], cs[j]
			if tapXOwnAnswer(fs.own, a, b) {
				// A forced tap (no ask: exactly the cost's candidates)
				// that takes the play's own {T} source is no continuation
				// (CR 602.2b), as the tap-another search's is skipped.
				continue
			}
			if colours(a, b) {
				return true
			}
		}
		return false
	}
	if nc > 1 && !fs.tick() { // the creature ask
		return false
	}
	for i := 0; i < nc; i++ {
		if tapXOwnAnswer(fs.own, cs[i], NoSlot) {
			continue // a forced tap of the play's own {T} source (CR 602.2b)
		}
		if colours(cs[i], NoSlot) {
			return true
		}
	}
	return false
}

// otherGap branches on gap sl's answers at a priority node of the search.
func (fs *filtSearch) otherGap(k *prep, t *State, sl Slot, funded int) bool {
	p := fs.p
	try := func(o otherStep) bool {
		return fs.try(k, t, filtStep{sl: sl, gap: true, other: o}, funded, true)
	}
	if found, handled := fs.otherOwnAsk(k, t, sl, funded, try); handled {
		return found
	}
	switch {
	case t.sacHolderSpent(k, sl):
		// the grant's tap is spent: the holder's printed sacrifice is its one
		// ability left, a gap step of its own with no ask (mana_sacholder.go)
		return fs.try(k, t, filtStep{sl: sl, gap: true, sacOnly: true}, funded, true)
	case t.mixComboAny(k, p, true).has(sl):
		return fs.comboAnyOther(t, k, sl, try)
	case t.mixEach(k, p, true).has(sl):
		return try(otherStep{kind: otherEach}) // one ability: no ask (eachSearch.visit's free gap)
	case t.scriptAnyR(k, p).has(sl):
		return fs.anyRGap(k, t, sl, try)
	case t.scriptTapX(k, p).has(sl):
		return fs.tapXGap(k, t, sl, try)
	case t.scriptTapsDefer(k, p).has(sl):
		return fs.tapsDeferGap(k, t, sl, funded)
	case t.canTapForMana(k, sl) || t.sacHolderGap(k, sl):
		// several abilities: the "choose a mana ability" ask, one node, the
		// printed ability's answers first (grantSrch.visit)
		var gi [maxGrants]int16
		ng := t.grantIdxs(k, sl, &gi)
		dual := t.ownManaUsable(k, sl)
		alts := ng
		if dual {
			alts++
		}
		if alts > 1 && !fs.tick() {
			return false
		}
		colours := func(g int16, mask, n uint8, ask bool) bool {
			if mask == 0 {
				mask = 1 << kop.C
			}
			if ask && mask&(mask-1) != 0 && !fs.tick() {
				return false
			}
			var order [kop.NumManaKinds]uint8
			no := t.stepColours(k, sl, g, mask, &order)
			needFirst(order[:no], fs.need) // gorge's scriptOrderAnswers on the same ask
			for _, c := range order[:no] {
				if try(otherStep{kind: otherGrant, ans: c, n: n, gi: g}) {
					return true
				}
			}
			return false
		}
		if dual && t.sacHolderGap(k, sl) {
			// the printed sacrifice answer: it floats the mana and sacrifices
			// the holder, so the grant is gone with it (mana_sacholder.go)
			if fs.try(k, t, filtStep{sl: sl, gap: true, sacOnly: true}, funded, true) {
				return true
			}
		} else if dual {
			pc := t.manaPCGen(k, sl)
			if colours(-1, t.srcMaskAt(pc, sl), pc.manaN, alts == 1) {
				return true
			}
		}
		d := t.grantPayDesc(k, fs.a)
		for j := 0; j < ng; j++ {
			if r := k.gen.grantOf[gi[j]]; k.gen.grantRestr[gi[j]] != 0 && !t.restrSrcAdmits(k, p, sl, &k.t.Cards[r.card].Abilities[r.ab], &d) {
				continue
			}
			pc := t.grantView(k.gen, gi[j], sl)
			if grantHasCost(pc) {
				continue // a life or sacrifice cost: skipped, not modelled here (this step only taps); the grant search's step pays it (mana_grantcost.go)
			}
			mask := t.srcMaskAt(pc, sl)
			if colours(gi[j], mask, pc.manaN, alts == 1 || mask&uint8(kop.ColAny) == uint8(kop.ColAny)) {
				return true
			}
		}
	}
	return fs.riderOther(k, t, sl, funded, try)
}

// anyRGap branches any-colour restricted gap sl (scriptAnyR) on its answers at
// a priority node, as the restricted-gap scripts' scriptDFS does. A wide
// second ability (mana_restrictedcombo.go) answers the first ability's colours
// and then its two-unit ask's multisets; a plain source answers its printed
// first mana ability, then the colour ask's six answers (anyRSteps).
func (fs *filtSearch) anyRGap(k *prep, t *State, sl Slot, try func(otherStep) bool) bool {
	if !fs.tick() {
		return false
	}
	c := t.cardAt(sl)
	if restrWideAnyMana(&k.t.Cards[c].Abilities[k.gen.anyR(c)]) {
		off := uint8(0)
		if t.wideSecond(k, sl) {
			// the first tick was the "choose a mana ability" ask; the
			// first ability's colours, then the allocation ask's node
			// (mana_restrictedcombo.go)
			if fs.wide2FirstColours(func(col uint8) bool { return try(otherStep{kind: otherAnyR, ans: col}) }) {
				return true
			}
			if !fs.tick() {
				return false
			}
			off = kop.NumManaKinds - 1
		}
		// the two-unit ask: gorge's 25 per-unit answers (scriptAnswers,
		// unit-major W W, W U ... G G), those naming a colour the play
		// announces first and engine order kept in each half (gorge's
		// scriptOrderAnswers on the same ask: its W-subtree spent the
		// search's budget before the first wanted pair). A mirrored pair (its
		// second unit below its first) makes the pool its twin, earlier in
		// the same half, made: it charges its node where gorge visits it and
		// finds the state seen. off shifts the multiset answers past the
		// first ability's colours when the gap is a second ability.
		for half := 0; half < 2; half++ {
			for a := 0; a < 5; a++ {
				for b := 0; b < 5; b++ {
					if (fs.need[a] || fs.need[b]) != (half == 0) {
						continue
					}
					if b < a {
						if !fs.tick() {
							return false
						}
						continue
					}
					if try(otherStep{kind: otherAnyR, ans: off + wideAnswerIndex(a, b)}) {
						return true
					}
				}
			}
		}
		return false
	}
	// its printed first mana ability first: gorge's "choose a mana
	// ability" ask has no ManaSymbol, so scriptOrderAnswers leaves it
	// alone (on Cavern of Souls et al. answer 0 is the printed {C}).
	// Then the colour ask (scriptDFS's six answers), gorge's
	// scriptOrderAnswers on it: a colour the play announces first, the
	// rest in engine order.
	if anyRSteps(fs.need, func(ans uint8) bool {
		return try(otherStep{kind: otherAnyR, ans: ans})
	}, fs.tick) {
		return true
	}
	return false
}

// tapsDeferGap branches Taps-deferred gap sl (Grove of the Burnwillows' rider,
// a depletion land) on its activation's answers at a priority node, as the
// tap-another search's deferAsk does: deferGapAnswers of them in the script
// search's order, the production ask a node when there are two or more.
func (fs *filtSearch) tapsDeferGap(k *prep, t *State, sl Slot, funded int) bool {
	n := t.deferGapAnswers(k, sl)
	if n > 1 && !fs.tick() {
		return false
	}
	for c := uint8(0); c < n; c++ {
		st := filtStep{sl: sl, gap: true, act: fs.a, other: otherStep{kind: otherTapsDefer, ans: c}}
		if fs.try(k, t, st, funded, true) {
			return true
		}
	}
	return false
}

// ctrOther branches counter-cost gap sl's answers at a priority node, as
// ctrScript's chain does: a source with a plain {T} first ability (ctrSecond,
// Weather Maker) answers the activation ask with that ability first, its
// colour ask over ctrFirstN's answers (ctrColOrder: the play's announced
// colours first) and only then the counter ability; a counter ability that
// asks a colour (Vivid Grove's second) answers it in the same order. Each ask
// is one node of the search, as gorge's activation ask is.
func (fs *filtSearch) ctrOther(k *prep, t *State, sl Slot, try func(otherStep) bool) bool {
	c := t.cardAt(sl)
	if k.ctrSecond(c) {
		if !fs.tick() {
			return false // the "choose a mana ability" ask
		}
		off := k.ctrFirstN(c)
		if off > 1 && !fs.tick() {
			return false // the plain first ability's colour ask
		}
		for ans := 0; ans < off; ans++ {
			col := int8(-1) // a fixed colour asks nothing
			if off > 1 {
				col = int8(t.ctrColOrder(k, fs.a)[ans])
			}
			if try(otherStep{kind: otherCtr, col: col, plain: true}) {
				return true
			}
		}
	}
	ab := k.ctrAbOf(c)
	if ab == nil {
		return false
	}
	if ctrAnyColour(ab) {
		// The prefix search's activation ask lists the source's abilities
		// (the plain {T} first, then the counter one) and the counter
		// ability's colour is a second ask, one more node of gorge's budget
		// (scriptSearch.visit; the payment window's flattening into one
		// option per colour, announce_pay.go AnnouncedAbilityColours, is not
		// this ask). A source with the one ability poses only the colour ask,
		// which the activation's own node already counts.
		if k.ctrSecond(c) && !fs.tick() {
			return false
		}
		order := t.ctrColOrder(k, fs.a)
		// The one-shot flexible gap's colour prune (gorge's oneShotAskPrune,
		// potential_script.go, cut D): while a fixed pip is owed, a colour
		// that fills none is dominated by one that does. Fail open when no
		// colour fills a pip (the prune never empties the ask).
		oneShot := fs.feeOnly && fs.ctrOneShot(k, t, sl, ab)
		if oneShot {
			any := false
			for _, col := range order {
				if !t.ctrOneShotColStalls(k, fs.p, sl, &fs.useful, int(col)) {
					any = true
					break
				}
			}
			oneShot = any
		}
		for _, col := range order {
			if oneShot && t.ctrOneShotColStalls(k, fs.p, sl, &fs.useful, int(col)) {
				continue
			}
			if try(otherStep{kind: otherCtr, col: int8(col)}) {
				return true
			}
		}
		return false
	}
	if kop.StoreManaAb(ab) {
		if n := t.perms[sl].ctrN(t, int(ab.Cost.Ctr)); n > 0 {
			// a storage battery's counter ability asks X (gorge's X ask, one
			// node, then every X from the counters held down to 0, each a
			// subtree: grantSrch.storeBranch is the grant search's twin)
			if !fs.tick() {
				return false
			}
			for x := min(n, 255); x >= 0; x-- {
				if try(otherStep{kind: otherCtr, col: -1, x: uint8(x), xset: true}) {
					return true
				}
			}
			return false
		}
	}
	return try(otherStep{kind: otherCtr, col: -1})
}

// ctrOneShot reports whether the counter-cost gap's colour ask is the
// one-shot flexible class (a gap that is not the fee-only repeatable filter,
// no mana fee, a fixed-pip cost read): gorge's oneShotAskPrune class.
func (fs *filtSearch) ctrOneShot(k *prep, t *State, sl Slot, ab *kop.Ability) bool {
	if !fs.usefulOK || !fs.other.has(sl) {
		return false
	}
	return ab.Cost.Mana.Generic == 0 && pipSum(&ab.Cost.Mana.Pips) == 0 && ab.Cost.Mana.Hyb == ([4]kop.ColorMask{})
}

// ctrOneShotColStalls: the counter-cost gap at sl making col fills no fixed
// pip the play still lacks while one is owed (gorge's oneShotAskPrune; dirac's
// filtOneShotColStalls is the filter-ability twin).
func (s *State) ctrOneShotColStalls(k *prep, p uint8, sl Slot, useful *kop.ManaCost, col int) bool {
	if useful == nil || useful.Hyb[0] != 0 || s.g.winMode == winTrig {
		return false
	}
	if col < 0 || col >= kop.NumManaKinds {
		return false
	}
	pool := s.pl[p].pool
	if m := s.restrPoolAdmits(k, p); m != 0 {
		s.rbUnits(p, m, &pool)
	}
	before := feeOnlyShort(&useful.Pips, &pool)
	if before == 0 {
		return false
	}
	pool[col]++
	return feeOnlyShort(&useful.Pips, &pool) >= before
}

// chosenPairGapSrc: gap sl is a chosen-colour pair source (mana_chosenfilter.go).
func (s *State) chosenPairGapSrc(k *prep, sl Slot) bool {
	return k.gen.rs != nil && k.gen.rs.chosenPair && s.chosenPairSrc(k, s.cardAt(sl))
}

// otherDo plays step st of a gap that is no filter.
func (s *State) otherDo(k *prep, p uint8, st filtStep) {
	o := &st.other
	switch o.kind {
	case otherDeferred:
		s.deferApply(k, p, st.sl, int(o.col)) // its {R} floats as the deferred source's (mana_grantdeferred.go); -1 the source's own colour
	case otherAnyR:
		var spill [kop.NumManaKinds]uint8
		s.tapGapChoice(k, p, st.sl, slotMask{}, slotMask{}.with(st.sl), slotMask{}, int(o.ans), &spill)
		if spill != ([kop.NumManaKinds]uint8{}) {
			s.capFull(StallRestrSrc) // never a silent spill
		}
	case otherRider:
		if s.riderPlainFirst(k, st.sl) {
			s.riderPlainGap(k, p, st.act, st.sl, int(o.ans)) // the plain first ability and the rider's colours are one ask (mana_rider.go)
			s.sba(k)
			return
		}
		var spill [kop.NumManaKinds]uint8
		s.tapGapChoice(k, p, st.sl, slotMask{}, slotMask{}, slotMask{}.with(st.sl), int(o.ans), &spill)
		if spill != ([kop.NumManaKinds]uint8{}) {
			s.capFull(StallRestrSrc) // never a silent spill
		}
	case otherTapX:
		s.tapXDo(k, p, tapXStep{sl: st.sl, cand: o.cand, cand2: o.cand2, col: o.ans})
	case otherTapXPlain:
		s.tapXPlainDo(k, p, st.sl, o.ans)
	case otherChosen:
		s.chosenGapDo(k, p, st.sl, o.plain)
	case otherTapsDefer:
		s.tapsDeferApply(k, p, st.act, st.sl, o.ans) // the play the search prices (st.act) orders the production ask's answers
	case otherComboAny:
		if k.isReflAmt(s.cardAt(st.sl)) {
			s.reflAmtOtherDo(k, p, st.sl, o.ans)
			break
		}
		s.caActivate(k, p, st.sl, int(o.ans))
	case otherEach:
		s.perms[st.sl].flags |= pfTapped
		s.touchSl(st.sl)
		s.manaTapped(k, st.sl)
		s.eachProduce(k, p, st.sl, 0)
	case otherLife:
		s.lifeGapDo(k, p, st.sl, int(o.ans))
	case otherLifePain:
		s.lifePainDo(k, p, st.sl, int(o.ans))
	case otherCtr:
		if o.plain {
			// the plain first {T} ability's answer: the counters and the use
			// stay, as ctrAb1 (gorge's activation: the {T} cost, then the mana)
			s.ctrAb1(k, p, st.sl, int(o.col))
			return
		}
		// remove counters, tap, add mana, spend the use (ctrGap); the SBA a
		// -0/-1 counter may kill with
		if o.xset {
			s.storeGapX(k, p, st.sl, k.ctrAbOf(s.cardAt(st.sl)), int(o.x)) // the X ask's answer
		} else {
			s.ctrGap(k, p, st.sl, int(o.col))
		}
		s.sba(k)
	case otherGrant:
		s.perms[st.sl].flags |= pfTapped
		s.touchSl(st.sl)
		s.manaTapped(k, st.sl)
		if o.gi >= 0 {
			if ab := restrAbOfGrant(k, o.gi); ab != nil {
				if !s.floatRestrAb(k, SeatIx(p), st.sl, ab, int(o.ans), o.n) {
					s.capFull(StallRestrSrc)
				}
				return
			}
		}
		s.pl[SeatIx(p)].pool[o.ans] += o.n
		// The tap brings its tap-extra too (gorge addTapExtra): a granted
		// source's is the grant's own extra (Honest Work's Humble Merchant
		// tapped under Badgermole Cub). The search priced the prefix without
		// it, so adding it can only leave the payment payable.
		if o.gi >= 0 {
			s.addExtra(k, SeatIx(p), st.sl, s.grantView(k.gen, o.gi, st.sl))
		} else {
			s.addExtra(k, SeatIx(p), st.sl, s.manaPCGen(k, st.sl))
		}
		s.artTag(k, p, st.sl, int(o.ans))
	}
}

// filtFeeRestrN is how much of p's floating restricted mana filter sl's fee may
// spend: the fee is an activated ability's payment (Hydraulic Helper's mana
// "can't be spent to cast a nonartifact spell", so it can pay it).
func (s *State) filtFeeRestrN(k *prep, p uint8, sl Slot) int {
	return s.rbTotal(p, s.filtFeeRestrM(k, p, sl))
}

// filtFeeRestrM is the mask of p's restricted batches filter sl's fee may spend.
func (s *State) filtFeeRestrM(k *prep, p uint8, sl Slot) uint8 {
	if !s.rbAny(p) {
		return 0
	}
	d := payDesc{class: pdAct, card: s.cardAt(sl), src: sl}
	return s.restrPoolAct(k, p, &d)
}

// filtFeeRestr spends up to fee of that mana and returns how much it spent:
// gorge carves the batches the payment may spend before ordinary mana.
func (s *State) filtFeeRestr(k *prep, p uint8, sl Slot, fee int) int {
	m := s.filtFeeRestrM(k, p, sl)
	return s.rbSpendN(p, m, min(fee, s.rbTotal(p, m)))
}

// filtFeeSpend pays up to fee of p's visible pool as gorge's payment spends
// it (filtDo's gap fee, windowFiltFirstFallback's rehearsal of it): the
// colours in C W U B R G order, the admitted batches' units in line with the
// plain ones and then carved by what their colour's spend actually consumed
// (ResolveMana's plain-first unit search, emitRestrictedManaSpend's cap; a
// typed batch's units are spent after the plain ones of their colour,
// rbSettle). The batches-first spend this replaces consumed a batch's units
// the payment had not reached, leaving the plain mana the fee had spent and
// handing the searched play plain units the payment refuses (Vedalken
// Engineer's artifact-only batch beside a Stonework Packbeast fee paid
// Donate's {2}). It returns the unpayable remainder.
func (s *State) filtFeeSpend(k *prep, p uint8, sl Slot, fee int) int {
	if fee <= 0 {
		return 0
	}
	m := s.filtFeeRestrM(k, p, sl)
	typed := m != 0 && s.rbTyped(p)
	for _, c := range [6]int{kop.C, kop.W, kop.U, kop.B, kop.R, kop.G} {
		if fee <= 0 {
			break
		}
		pl := &s.pl[SeatIx(p)]
		var batch int
		if m != 0 {
			for i := range s.rb[SeatIx(p)] {
				if m&(1<<i) != 0 {
					batch += int(s.rb[SeatIx(p)][i].n[c])
				}
			}
		}
		need := min(fee, int(pl.pool[c])+batch)
		if need <= 0 {
			continue
		}
		carve := min(need, batch) // a plain batch's units carve the colour's spend first
		if typed {
			carve = need - min(need, int(pl.pool[c])) // a typed batch's units are spent after the plain ones
		}
		applied := 0
		for i := range s.rb[SeatIx(p)] {
			if m&(1<<i) == 0 || carve == 0 {
				continue
			}
			b := &s.rb[SeatIx(p)][i]
			d := min(carve, int(b.n[c]))
			b.n[c] -= uint8(d)
			carve -= d
			applied += d
		}
		pl.pool[c] -= uint8(need - applied)
		fee -= need
	}
	s.rbCompact(p)
	if !s.rbAny(p) {
		s.g.poolArt[SeatIx(p)] &^= poolArtRestr
	}
	return fee
}

// wideAnswerIndex is the scriptDFS answer of the two-colour multiset a <= b,
// wideAnswer's inverse: the multisets in order W W, W U ... W G, U U ... G G.
func wideAnswerIndex(a, b int) uint8 {
	return uint8(a*5 - a*(a-1)/2 + b - a)
}
