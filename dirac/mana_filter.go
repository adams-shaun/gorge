package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Filter mana abilities (CR 605.3a, 601.2g): a mana ability with a mana
// cost, Hidden Grotto's "{1}, {T}: Add one mana of any color" beside its
// free "{T}: Add {C}". gorge's planner census prices tap-only abilities
// (rules/pay appendUnitAlternatives: TierNormal needs TapOnly), so the
// filter is a paid census gap (potential_plan.go paymentPlanCensusOf):
//
//   - a play the planner proves insufficient is re-asked with the filter
//     as a free any-colour unit and its fee charged as generic
//     (paymentPlanRelaxProof); one that relaxed run pays takes one of the
//     potentialScriptPlays prefix searches (availFilt, scriptOffers);
//   - the search (potentialPrefixScript -> scriptSearch.visit, gapsOnly
//     with funding = the fees) activates, depth first in priority-option
//     (battlefield) order, the gaps first and then up to fees other
//     sources, branching on every ask; after a gap was activated the
//     planner is asked at each node. The play is offered when a path is
//     found within potentialScriptNodes, and the oracle plays the path
//     before the play (prScript), which the planner then pays from the
//     floating pool (filtSearch).
//
// The kernel runs the same search on State copies, the plain walk standing
// for the planner, and plays the found path at the play (scriptPrefix).
// Only boards whose census gaps are all filters take it; a filter beside a
// Bloom Tender or a restricted gap keeps the older scripts' logic.

const potentialScriptNodes = 160 // gorge rules/potential_script.go

// paySearchNodes bounds a payment plan's search. A payment taps the cost's own
// sources beside the filters' fees, so its tree is wider than a play prefix's;
// the fixed-source-first funding order (filtSearch.visit) keeps ordinary
// boards under it. It is a declared capacity bound: a search past it declines.
const paySearchNodes = 1024

// relaxedPaidLimit bounds the paid census gaps gorge's relaxed proof
// enumerates (rules/potential_plan.go): with a fourth filter the census is
// unrelaxable and its paid list empty, so no relaxed proof runs (every
// insufficient play is searched) and the search funds nothing (fees() 0):
// it activates the filters alone.
const relaxedPaidLimit = 3

// filterTapMana: a, a mana ability of cd, is a filter the kernel models:
// {N}, {T} (generic N only), one mana of any colour, a dynamic amount of
// one colour (Three Tree City's {2},{T}), or a literal amount above one in
// any combination (Baxter Building's {4},{T}: four mana, a Combo Any
// allocation; mana_filter.go's filtCombo), no other cost or restriction, on
// a card whose first mana ability is a free {T} one. A filter that is the
// card's first mana ability (Three Tree Mascot's untapped {1}, Prophetic
// Prism) is kop.FilterManaAb's (mana_filterfirst.go).
func filterTapMana(cd *kop.CardDef, a *kop.Ability) bool {
	m := &a.Cost.Mana
	_, pip := fixedPipFilt(a)        // a gated one-pip fixed-colour filter (mana_filterpip.go)
	hyb := kop.HybridFilterManaAb(a) // a one-hybrid-pip fee, two of one chosen colour (mana_filterhybrid.go)
	if a.Kind != kop.AbMana || a.AFlags&^(kop.AfManaDynamic|kop.AfManaChoice) != 0 || a.ManaRestrict != 0 || a.Cost.Flags != kop.CostTap ||
		m.X != 0 || m.Hyb[0] != 0 && !hyb || a.ManaN == 0 {
		return false
	}
	if !pip && !hyb && (m.Generic == 0 || m.Pips != ([kop.NumManaKinds]uint8{}) || a.ManaColors != kop.ColAny || a.ManaN > 1 && (a.AFlags&kop.AfManaChoice == 0 || !comboFiltAb(a))) {
		return false
	}
	f := firstManaAb(cd)
	if f < 0 || &cd.Abilities[f] == a {
		return false
	}
	fa := &cd.Abilities[f]
	return fa.Cost.Flags == kop.CostTap && fa.Cost.Mana.MV() == 0 && fa.Cost.Mana.X == 0 && fa.AFlags == 0 && fa.ManaRestrict == 0
}

// comboFiltAb reports whether a produces its literal amount of mana as a
// combination (Produced$ Combo Any | Amount$ 4), so one activation makes
// Ab.ManaN mana split over any of the five colours.
func comboFiltAb(a *kop.Ability) bool {
	return a.AFlags&kop.AfManaChoice != 0 && a.ManaColors == kop.ColAny && a.ManaN > 1
}

// prepTapFilt records ability j of card c as the card's modelled filter
// (genPrep.filt, gfFilt): a dynamic amount is one that may resolve to zero,
// which scriptFilt then drops (gorge's census, manaAbilityYieldsNothing);
// a gated one-pip filter is mana_filterpip.go's.
func (g *genPrep) prepTapFilt(c CardID, j int, a *kop.Ability) {
	g.smallFor(c).filt = int8(j)
	g.setFlag(c, gfFilt)
	g.anyFilt = true
	if a.AFlags&kop.AfManaDynamic != 0 {
		g.anyDynFilt = true
	}
	if _, pip := fixedPipFilt(a); pip {
		g.anyFiltPip = true
	}
}

// prepCensusGap records ability j of card c as a census gap the kernel models
// beside the filter (genPrep.gapOther, gfGapOther); a dynamic amount is one
// that may resolve to zero, which gapOthers then drops (gorge's census,
// manaAbilityYieldsNothing).
func (g *genPrep) prepCensusGap(c CardID, j int, a *kop.Ability) {
	g.smallFor(c).gapOther = int8(j)
	g.setFlag(c, gfGapOther)
	if a.AFlags&kop.AfManaDynamic != 0 {
		g.anyDynGap = true
	}
	if kop.GapLifeManaAb(a) {
		g.uses.set(useLifeGap)
	}
}

// scriptFilt: p's activatable filter sources (untapped), a slot mask.
func (s *State) scriptFilt(k *prep, p uint8) slotMask {
	if !k.gen.anyFilt {
		return slotMask{}
	}
	var m slotMask
	if k.gen.anyFiltFirst {
		m = s.filtFirstMask(k, p) // a filter first and only (mana_filterfirst.go)
	} else {
		m = s.gapMask(k, p, gfFilt)
	}
	if k.gen.anyFiltPip {
		for b := m; b.any(); b = b.rest() {
			if sl := b.first(); !s.filtGateOn(k, sl) {
				m = m.without(sl) // a counters-gated filter is no gap while its gate fails (mana_filterpip.go)
			}
		}
	}
	// A filter whose fee an activated-ability reducer takes to nothing is the
	// source's own mana (mana_filterfree.go's gate-on view): it stays out of
	// the gap set on both paths. Otherwise a table holding any first-ability
	// filter (anyFiltFirst) would leave a later free filter in it, and the
	// prefix search spends its 160-node budget on the extra branch instead of
	// funding the paid filters beside it.
	if k.gen.anyAbReduce {
		for b := m; b.any(); b = b.rest() {
			if sl := b.first(); k.gen.has(s.cardAt(sl), gfFiltFee) && s.filtFee(k, sl) == 0 {
				m = m.without(sl) // free: part of the source's own mana (mana_filterfree.go)
			}
		}
	}
	// A filter whose amount resolves to zero makes nothing (gorge's
	// manaAbilityYieldsNothing: Baldur's Gate's "X mana of any one color, X =
	// other Gates" beside no other Gate). The census lists no ability of it,
	// so the prefix search must not fund its fee for no mana: the funding is
	// one covered activation per fee-bearing ability, and an unfunded zero
	// filter starves the search of the plays the census does pay.
	if k.gen.anyDynFilt {
		for b := m; b.any(); b = b.rest() {
			if sl := b.first(); s.filtUnits(k, sl) == 0 {
				m = m.without(sl) // the amount is zero: the ability yields nothing
			}
		}
	}
	return m
}

// FilterTapMana reports whether a, a mana ability of cd, is a filter the
// kernel models (filterTapMana).
func FilterTapMana(cd *kop.CardDef, a *kop.Ability) bool { return filterTapMana(cd, a) }

// filtFee is the filter's {N} at slot sl, after the cost reductions that
// reach it: a mana ability is an activated ability (CR 605.1a), so an
// activated-ability reducer lowers it (CR 601.2f, 602.2b; gorge
// manaAbilityCC).
func (s *State) filtFee(k *prep, sl Slot) int {
	c := s.cardAt(sl)
	a := &k.t.Cards[c].Abilities[k.gen.filt(c)]
	if _, ok := s.filtPip(k, sl); ok {
		return 1 // the pip: a generic reducer cannot lower it
	}
	if _, _, ok := s.filtHyb(k, sl); ok {
		return 1 // the hybrid pip: a generic reducer cannot lower it either
	}
	if s.filtRepeat(k, sl) {
		return s.filtRepeatFee(k, sl) // a fee-only one: its pip and its reduced generic part
	}
	if k.gen.anyAbReduce {
		return int(s.abReduced(k, a, sl, s.perms[sl].ctrl, a.Cost.Mana).Generic)
	}
	return int(a.Cost.Mana.Generic)
}

// filtUnits is the mana the filter at sl makes.
func (s *State) filtUnits(k *prep, sl Slot) int {
	a := s.filtAb(k, sl)
	if a.AFlags&kop.AfManaDynamic != 0 {
		return s.dynManaN(k, a, sl)
	}
	if comboFiltAb(a) {
		return int(a.ManaN)
	}
	if kop.HybridFilterManaAb(a) {
		return int(a.ManaN) // two of the one colour chosen
	}
	if a.ManaEach {
		n := 0
		for _, v := range filtEachVec(a) {
			n += int(v)
		}
		return n // an all-at-once filter's whole vector (mana_filtereach.go)
	}
	if a.AFlags&kop.AfManaChoice == 0 {
		return int(max(a.ManaN, 1)) // a pip-fee filter's N of its one chosen colour
	}
	return 1
}

// filtRelaxable: gorge's relaxed proof can enumerate the filters f: at most
// relaxedPaidLimit, each dynamic amount at least one and at most six
// (rules/potential_plan.go paymentPlanRelaxedAlternatives).
func (s *State) filtRelaxable(k *prep, f slotMask) bool {
	if f.count() > relaxedPaidLimit || s.filtRepeatIn(k, f) || s.filtHybIn(k, f) { // (a hybrid fee or a repeatable one has no relaxed alternative: mana_filterhybrid.go, gorge ok=false)
		return false
	}
	for ; f.any(); f = f.rest() {
		if !s.filtUnitsEnumerable(k, f.first()) {
			return false
		}
	}
	return true
}

// filtUnitsEnumerable: gorge's relaxed proof can enumerate the filter at sl's
// production: at least one unit, and at most six when the colour is a choice
// (relaxedAddProduction lists every multiset of an any-colour amount, and
// refuses one above six). A fixed-colour amount is one vector however large:
// Magus of the Coffers' {B} for each of seven Swamps is still relaxable.
func (s *State) filtUnitsEnumerable(k *prep, sl Slot) bool {
	n := s.filtUnits(k, sl)
	if n < 1 {
		return false
	}
	if a := s.filtAb(k, sl); a.AFlags&kop.AfManaChoice == 0 && bits.OnesCount8(uint8(a.ManaColors)) == 1 {
		return true
	}
	return n <= 6
}

// availFilt: in the gaps walk each filter is a free unit of its colours
// (any, or a Combo's) and its fee is charged on top of the cost (paymentPlanRelaxProof's run with
// every paid gap: the most permissive of its subsets, since an any-colour
// unit only helps). Charging the fee is taking it off the total.
//
//go:noinline
func (s *State) availFilt(k *prep, p uint8, a *avail) {
	neg := 0
	for b := s.scriptFilt(k, p); b.any(); b = b.rest() {
		units, fee := s.filtUnits(k, b.first()), s.filtFee(k, b.first())
		if units < fee {
			// a subset of the relaxed proof (mana_filtersubset.go): this filter is left out
			if neg++; neg <= relaxedPaidLimit && s.g.availX&availGaps != 0 && s.filtSkip()&(uint8(1)<<(neg-1)) != 0 {
				continue
			}
		}
		addFiltUnits(a, s.filtAb(k, b.first()), uint8(units))
		xv := s.filtExtra(k, b.first()) // the TapsForMana extra its {T} brings (Wolfwillow Haven on a Desolate Mire)
		for c, v := range xv {
			if v != 0 {
				a.addUnits(1<<c, v)
			}
		}
		a.total -= fee
	}
}

// faceReachable: the hybrid face r (plus extra generic) can be paid from av,
// or by activating some of p's filters in the mana window (CR 601.2g): each
// makes a coloured mana in place of its free ability's and takes its fee out
// of the other mana (CR 601.2h).
//
//go:noinline
func (s *State) faceReachable(k *prep, p uint8, av *avail, r *kop.ManaCost, extra int) bool {
	if av.affords(r, pipMaskOf(r), extra) {
		return true
	}
	if k.restrSrc && s.restrFaceAffords(k, p, r, av, extra) {
		return true
	}
	if k.gen != nil && k.gen.anyMultiR && s.wideFaceAffords(k, p, r, av, extra) {
		return true
	}
	if k.gen == nil || !k.gen.anyFilt {
		return false
	}
	var filt [maxFaceFilters]Slot
	n := 0
	own := NoSlot
	if e := &s.bld().e; e.kind == seAbility && k.t.Cards[e.card].Abilities[e.ab].Cost.Flags&kop.CostTap != 0 {
		own = s.srcSlot() // tapped for mana, it could not pay its own {T}
	}
	for b := s.scriptFilt(k, p); b.any() && n < len(filt); b = b.rest() {
		if sl := b.first(); sl != own {
			filt[n] = sl
			n++
		}
	}
	base := s.restrActFiltAvail(k, p, av)
	for set := 1; set < 1<<n; set++ {
		if s.faceReachSet(k, p, r, extra, base, &filt, 0, n, uint32(set)) {
			return true
		}
	}
	return false
}

// faceReachSet walks the set's filters over t, the avail the window starts
// with: each activated filter is removed, its fee comes first out of what the
// window has made so far (CR 605.3b) -- the units the fee spends come out of
// t's groups too, each way of spending it tried, so a unit that fed the fee
// pays no pip of its colour after (a floating B that funded Sunscorched
// Divide's {1} does not also pay a {B} pip) -- then the filter's production
// joins t and the walk goes on to the set's next filter. When no group holds
// a unit the fee can take, and on a board with painful units held out of the
// groups (and restricted units held out of a filter's own fee, CR 106.6),
// the fee reads off the total alone. It reports whether the set's last avail
// affords r.
func (s *State) faceReachSet(k *prep, p uint8, r *kop.ManaCost, extra int, t avail, filt *[maxFaceFilters]Slot, i, n int, set uint32) bool {
	for ; i < n; i++ {
		if set&(1<<i) == 0 {
			continue
		}
		sl := filt[i]
		if !s.filtNoTap(k, sl) && s.canTapForMana(k, sl) {
			pc := s.manaPCGen(k, sl)
			if !s.filterOwnTakes(k, &t, sl, pc) {
				// The filter's own production was already spent as an earlier
				// fee's mana: the permanent's one tap made it (CR 605.1a), so
				// the filter is spent, not producing.
				continue
			}
		}
		fee := s.filtFee(k, sl)
		feeCol := s.feeColour(k, sl)
		var held [kop.NumManaKinds]uint8
		if s.rbAny(p) {
			s.rbUnits(p, s.restrPoolAdmits(k, p)&^s.filtFeeRestrM(k, p, sl), &held) // restricted mana the payment may spend but the filter's own activation may not (CR 106.6)
		}
		held = dropHeld(&t, held)
		nh := 0
		for _, n := range held {
			nh += int(n)
		}
		if t.total < fee {
			return false
		}
		if fee == 0 || feeCol == 0 && (t.pain.on || nh != 0 || t.nOne != 0) {
			// the total-only reading: the painful and held-out units are out
			// of the groups, so the groups cannot price the fee; a one-colour
			// source's units sit in a ColAny group its entry in t.one counts
			// whole, so a unit taken from the group alone would leave the two
			// out of step. A coloured fee (feeCol) stays out of it: the total
			// alone holds no colour, and only a unit of the fee's colours
			// pays it.
			t.total -= fee
			if nh != 0 && t.total == 0 {
				// the fee took every unit that could pay it: none stays for the face
				t.cnt = [16]uint16{}
				t.nOne = 0
			}
			return s.faceReachProduce(k, p, r, extra, t, held, sl, filt, i+1, n, set)
		}
		return s.faceFeeSpend(k, p, r, extra, t, fee, 0, held, sl, filt, i+1, n, set, feeCol)
	}
	if t.affords(r, pipMaskOf(r), extra) {
		return true
	}
	if k.restrSrc {
		// the restricted units are the payment's, so the walk's fees left
		// them out (restrActFiltAvail); they pay the face it ends on (CR
		// 106.6). restrSrcAvail directly: the reserved census unit makes t's
		// total understate what the widened sources still make, which
		// restrSrcNone's shortcut would read as no help.
		if d := s.payDescNow(NoObj); d.class != pdAct {
			u, multi, nm := s.restrSrcAvail(k, p, d, &t, NoSlot)
			if u.affordsMulti(r, extra, multi[:nm]) {
				return true
			}
		}
	}
	return false
}

// feeColour is the colour mask of the filter at sl's coloured fee unit, 0 when
// any mana pays its fee: a pip fee's colour (mana_filterpip.go), a hybrid
// fee's two colours (mana_filterhybrid.go), and the pip inside a repeatable
// fee-only filter's generic-plus-pip fee (mana_filterrepeat.go).
func (s *State) feeColour(k *prep, sl Slot) uint8 {
	if c, ok := s.filtPip(k, sl); ok {
		return 1 << c
	}
	if a, b, ok := s.filtHyb(k, sl); ok {
		return 1<<uint(a) | 1<<uint(b)
	}
	if c, ok := s.filtRepeatPip(k, sl); ok {
		return 1 << c
	}
	return 0
}

// faceFeeSpend takes owed units of the filter's fee out of t's groups, from
// group index from on, each way of spending it tried, then goes on with the
// set's remaining filters over what is left (faceReachProduce). A coloured
// fee's unit (feeCol) comes only from a group of the fee's colours: no other
// unit pays it (gorge's window pays the fee through the ordinary mana payment,
// CR 605.3b), so when the groups hold none the fee reads off the total only if
// a painful group of the fee's colours can fund it -- the painful units sit
// out of the groups under the budget. A wide fee with no group holding a unit
// reads off the total alone, as on a board with painful units held out of the
// groups.
func (s *State) faceFeeSpend(k *prep, p uint8, r *kop.ManaCost, extra int, t avail, owed, from int, held [kop.NumManaKinds]uint8, sl Slot, filt *[maxFaceFilters]Slot, i, n int, set uint32, feeCol uint8) bool {
	if owed == 0 {
		return s.faceReachProduce(k, p, r, extra, t, held, sl, filt, i, n, set)
	}
	if feeCol != 0 {
		// the coloured unit first: only a group of the fee's colours holds
		// it, and the rest of the fee (a repeatable filter's generic part)
		// reads wide over every group
		for g := 0; g < t.n; g++ {
			if t.cnt[g] == 0 || t.mask[g]&feeCol == 0 {
				continue
			}
			u := t
			u.removeWide(u.mask[g], 1)
			if s.faceFeeSpend(k, p, r, extra, u, owed-1, 0, held, sl, filt, i, n, set, 0) {
				return true
			}
		}
		if !s.faceFeePainCol(t, feeCol) {
			return false // no unit of the fee's colours exists: the fee is unpayable
		}
		t.total -= owed
		return s.faceReachProduce(k, p, r, extra, t, held, sl, filt, i, n, set)
	}
	found := false
	for g := from; g < t.n; g++ {
		if t.cnt[g] == 0 {
			continue
		}
		found = true
		u := t
		u.removeWide(u.mask[g], 1)
		if s.faceFeeSpend(k, p, r, extra, u, owed-1, g, held, sl, filt, i, n, set, feeCol) {
			return true
		}
	}
	if found {
		return false // the fee's units are gone from the groups: none is left to spend
	}
	if t.total < owed {
		return false
	}
	t.total -= owed
	return s.faceReachProduce(k, p, r, extra, t, held, sl, filt, i, n, set)
}

// faceFeePainCol reports whether a painful group of the fee's colours can fund
// the fee: the painful units are held out of the groups (painTight) but they
// are real mana (gorge's pool reads them as any other), so a unit of the fee's
// colours there pays a coloured fee as the total-only reading of a wide fee
// does.
func (s *State) faceFeePainCol(t avail, feeCol uint8) bool {
	pa := &t.pain
	if !pa.on {
		return false
	}
	for g := uint8(0); g < pa.n; g++ {
		if pa.cnt[g] != 0 && pa.mask[g]&feeCol != 0 {
			return true
		}
	}
	return false
}

// faceReachProduce re-adds the held-out restricted mana (it may pay the face,
// only not this filter's own fee, CR 106.6), joins the filter's production and
// goes on with the set's remaining filters.
func (s *State) faceReachProduce(k *prep, p uint8, r *kop.ManaCost, extra int, t avail, held [kop.NumManaKinds]uint8, sl Slot, filt *[maxFaceFilters]Slot, i, n int, set uint32) bool {
	for c, n := range held {
		if n != 0 {
			t.addUnits(1<<c, n)
		}
	}
	addFiltUnits(&t, s.filtAb(k, sl), uint8(s.filtUnits(k, sl)))
	return s.faceReachSet(k, p, r, extra, t, filt, i, n, set)
}

// StallFiltX: an {X} announcement on a board with more activatable filters than
// filtMaxX tries in combination (maxFaceFilters).
const StallFiltX = 68

func init() { stallNames[StallFiltX] = "filtx" }

// filtMaxX is the largest X p's mana window can pay for cost once some of p's
// filters are activated in it, -1 when none pays X = 0. X is announced before
// the window opens and bounded by what it can make (CR 601.2b, 601.2g): gorge's
// xAsk prices each X through castWindowPays, whose search funds a filter's fee
// from mana made earlier in the window (CR 605.3b), so Starfleet Academy's
// {5},{T} turns five lands into as many mana as p has creature types. Each set
// of filters is tried, as in faceReachable; a filter making several mana makes
// them in one colour.
//
//go:noinline
func (s *State) filtMaxX(k *prep, p uint8, cost *kop.ManaCost, av *avail, anyColor bool) int {
	var filt [maxFaceFilters]Slot
	n := 0
	own := NoSlot
	if e := &s.bld().e; e.kind == seAbility && k.t.Cards[e.card].Abilities[e.ab].Cost.Flags&kop.CostTap != 0 {
		own = s.srcSlot() // tapped for its own cost, it makes no mana
	}
	for b := s.scriptFilt(k, p); b.any(); b = b.rest() {
		sl := b.first()
		if sl == own {
			continue
		}
		if n == len(filt) {
			s.capFull(StallFiltX)
			break
		}
		filt[n] = sl
		n++
	}
	if k.restrSrc {
		// the restricted sources the announced payment may spend (a
		// Powerstone's {C}, CR 106.6) count beside the filters: an
		// ability's fund a filter's fee, itself an activation, as any other
		// mana, and a spell's the artifact spell itself, so the larger X is
		// the filters' net plus them (a Powerstone pays the spell, a basic
		// the Bog's fee)
		d := s.payDescNow(NoObj)
		skip := NoSlot
		if d.class == pdAct {
			skip = d.src
		}
		t, _, _ := s.restrSrcAvail(k, p, d, av, skip)
		av = &t
	}
	best := -1
	w := filtWalk{s: s, k: k, p: p, filt: &filt, cost: cost, anyColor: anyColor}
	// An announced tap-paid pick pays one pip or one generic of the folded
	// cost, so the bound prices the pick-folded cost the ask prices (gorge
	// xAsk, paymentManaX): without it the bound stops one X short of what
	// the window with the filter can pay. The applied-pick arm is the same
	// cost as the primary arm wherever that arm absorbs, so it covers both
	// the ask's offers and its fallback offers. A cost modifier that reads
	// the spell's mana value sees X and can make a larger X cheaper
	// (cast_xreduced.go xCostDependsOnX), which breaks the sweep's monotone
	// break, so the fold is installed only when no modifier reads X.
	wbX := s.waterbendXBuilding(k) && s.waterbendPickCount() > 0
	if !s.xCostDependsOnX(k) && (s.convokePickCount() > 0 || wbX) {
		w.fold = func(x int) (kop.ManaCost, bool) { return s.xFoldCost(k, *cost, x, wbX, false) }
	}
	for set := 1; set < 1<<n; set++ {
		best = max(best, w.activate(*av, filtPos{set: set}))
	}
	return best
}

// filtBoundExtra is what p's filters add to gorge's X ceiling past their free
// taps: xAsk's bound sums each source's largest production, a filter's fee aside.
func (s *State) filtBoundExtra(k *prep, p uint8) int {
	if k.gen == nil || !k.gen.anyFilt {
		return 0
	}
	n := 0
	for b := s.scriptFilt(k, p); b.any(); b = b.rest() {
		sl := b.first()
		own := 0
		if !s.filtNoTap(k, sl) && s.canTapForMana(k, sl) {
			own = int(s.manaPCGen(k, sl).manaN)
		}
		n += max(0, s.filtUnits(k, sl)-own)
	}
	return n
}

// dropHeld removes held's units from t, capped by what each colour's group
// still holds: an earlier fee in the same walk may already have spent part of
// a batch. It returns what it actually removed, which the walk restores after
// the fee (a full held mask would resurrect units a fee already spent).
func dropHeld(t *avail, held [kop.NumManaKinds]uint8) [kop.NumManaKinds]uint8 {
	var dropped [kop.NumManaKinds]uint8
	for c, want := range held {
		if want == 0 {
			continue
		}
		mask := uint8(1) << c
		for gi := 0; gi < t.n; gi++ {
			if t.mask[gi] != mask {
				continue
			}
			d := min(uint16(want), t.cnt[gi])
			t.cnt[gi] -= d
			t.total -= int(d)
			dropped[c] = uint8(d)
			break
		}
	}
	return dropped
}

// filtFeeHeld is the restricted floating mana the payment may spend but
// filter sl's fee may not (CR 106.6): the same batches faceReachSet holds out
// of the fee, so the X bound prices a filter activation with the mana the real
// payment can give it.
func (s *State) filtFeeHeld(k *prep, p uint8, sl Slot) [kop.NumManaKinds]uint8 {
	var held [kop.NumManaKinds]uint8
	if !s.rbAny(p) {
		return held
	}
	s.rbUnits(p, s.restrPoolAdmits(k, p)&^s.filtFeeRestrM(k, p, sl), &held)
	return held
}

// filtWalk is one activation of a set of filters on the window's mana (CR
// 605.3b): filtMaxX runs it per set and it prices each result. fold, when
// the entry being built carries announced tap-paid picks, is the cost the
// ask prices at x (lockstepconfig.go xFoldCost): the bound then prices the
// pick-folded cost, exactly the cost the ask's own sweep prices past mx.
type filtWalk struct {
	s        *State
	k        *prep
	p        uint8
	filt     *[maxFaceFilters]Slot
	cost     *kop.ManaCost
	anyColor bool
	fold     func(x int) (kop.ManaCost, bool)
}

// filtPos is where a walk stands: the filters still to activate, the sweep
// (pass 0 takes the filters that add at least what they cost, pass 1 the
// rest), the next filter to try in it and whether this sweep activated one.
type filtPos struct {
	set, pass, i int
	progress     bool
}

// activate activates the filters of pos.set on t: each gives up its free
// ability's mana, pays its fee out of the mana made so far and adds its own.
// The fees are paid in an order that funds them, the filters that add at
// least what they cost first. It returns the largest X any way of paying the
// fees leaves, -1 when some fee cannot be paid.
func (w *filtWalk) activate(t avail, st filtPos) int {
	s, k, filt := w.s, w.k, w.filt
	for {
		if st.i >= len(filt) {
			if st.progress && st.set != 0 {
				st.i, st.progress = 0, false
				continue
			}
			if st.pass == 1 || st.set == 0 {
				break
			}
			st.pass, st.i, st.progress = 1, 0, false
			continue
		}
		i := st.i
		st.i++
		if st.set&(1<<i) == 0 {
			continue
		}
		sl := filt[i]
		var pc *pcard
		if !s.filtNoTap(k, sl) && s.canTapForMana(k, sl) {
			pc = s.manaPCGen(k, sl)
		}
		ownN := 0
		if pc != nil {
			ownN = int(pc.manaN)
		}
		fee, units := s.filtFee(k, sl), s.filtUnits(k, sl)
		// The fee is an activated ability's payment: restricted floating mana
		// the cast may spend but this activation may not stays out of it
		// (CR 106.6), exactly as faceReachSet holds it out of the face walk.
		// base restores it on every arm that leaves this filter unactivated.
		base := t
		held := dropHeld(&t, s.filtFeeHeld(k, w.p, sl))
		if st.pass == 0 && units < ownN+fee || t.total-ownN < fee {
			t = base
			continue
		}
		if col, ok := s.filtFeePip(k, sl); ok && !t.canMake(col) {
			t = base
			continue // no mana made so far can be the fee's {c}: another filter may make it first
		}
		if a, b, ok := s.filtHyb(k, sl); ok && !t.canMakeAny(uint8(1)<<uint(a)|uint8(1)<<uint(b)) {
			t = base
			continue // no mana made so far can be the fee's {a/b}: another filter may make one first
		}
		if pc != nil {
			if !s.filterOwnTakes(k, &t, sl, pc) {
				// The filter's own production was already spent as an earlier
				// fee's mana: the permanent's one tap made it (CR 605.1a), so
				// the filter is spent, not producing. Leave the total and go
				// on with the set's rest.
				st.set &^= 1 << i
				st.progress = true
				t = base
				continue
			}
		}
		st.set &^= 1 << i
		st.progress = true
		// The fee is generic mana, but the units it spends are gone from the
		// colours the rest of the cost can use (a floating U that fed a Signet
		// no longer pays a {U} pip), and only from what was made before this
		// filter: its own mana cannot fund its fee. Each way of spending it
		// is tried; a board with painful units held out of the groups keeps
		// the total-only reading.
		if col, ok := s.filtFeePip(k, sl); ok {
			return w.spendPip(t, st, sl, fee, col, held) // the fee's {c} needs mana that can be c (CR 107.4)
		}
		if a, b, ok := s.filtHyb(k, sl); ok {
			// the fee's {a/b} needs mana that can be a or b (CR 107.4c): the
			// hybrid fee is no generic part (gorge's castWindowPays funds it
			// through the ordinary pip payment, mana_filterhybrid.go
			// filtPayHyb), so spendPip's colour set is the two faces'
			return w.spendPipMask(t, st, sl, fee, uint8(1)<<uint(a)|uint8(1)<<uint(b), held)
		}
		if fee == 0 || t.pain.on || t.nOne != 0 { // (a one-colour source's group and entry move together or not at all)
			t.total -= fee
			return w.activated(t, st, sl, held)
		}
		return w.spendFee(t, st, sl, fee, 0, held)
	}
	if st.set != 0 {
		return -1
	}
	if w.fold != nil {
		return w.foldedMaxX(t)
	}
	return t.maxXScore(w.cost, w.anyColor)
}

// foldedMaxX is activate's terminal X sweep when the entry carries announced
// tap-paid picks: gorge's xAsk prices every X through paymentManaX, whose
// fold applies the announced picks (lockstepconfig.go xFoldCost), so the
// bound prices the same folded cost -- a convoke pick's pip or generic is
// mana the announced X no longer needs, and the larger X the window (its
// filters' fees funded, CR 605.3a) can still make must be offered (Chord of
// Calling announced with two picks beside a Desolate Mire's {1},{T}).
// filtMaxX installs the fold only when no cost modifier reads the announced
// X (cast_xreduced.go xCostDependsOnX), so the folded cost only grows with
// x: the sweep breaks at the first value the post-filter census cannot pay.
// A value whose Waterbend<X> picks exceed x (ok=false) is skipped, not a
// break, since a later, larger X can absorb them.
func (w *filtWalk) foldedMaxX(t avail) int {
	best := -1
	for x := 0; x < maxXAsk; x++ {
		c, ok := w.fold(x)
		if !ok {
			continue
		}
		if w.anyColor {
			if !t.affordsAnyColor(&c, 0) {
				break
			}
		} else if !t.affords(&c, pipMaskOf(&c), 0) {
			break
		}
		best = x
	}
	return best
}

// filtFeePip is the coloured pip of the filter at sl's fee, whether it is a
// printed pip-fee filter (Apprentice Wizard's {U}, {T}) or a repeatable one.
func (s *State) filtFeePip(k *prep, sl Slot) (int, bool) {
	if col, ok := s.filtPip(k, sl); ok {
		return col, true
	}
	return s.filtRepeatPip(k, sl)
}

// canMake: some unit of a can be colour col.
func (a *avail) canMake(col int) bool {
	for g := 0; g < a.n; g++ {
		if a.cnt[g] != 0 && a.mask[g]&(1<<uint(col)) != 0 {
			return true
		}
	}
	return false
}

// canMakeAny: some unit of a can be a colour of mask m (a hybrid fee's two
// faces: CR 107.4c, either pays the pip).
func (a *avail) canMakeAny(m uint8) bool {
	for g := 0; g < a.n; g++ {
		if a.cnt[g] != 0 && a.mask[g]&m != 0 {
			return true
		}
	}
	return false
}

// spendPip pays the fee pip of colour col out of a group that can make col
// (gorge's castWindowPays funds {U} with blue mana, not any mana), each such
// group tried, then the rest of the fee (fee-1 generic) as spendFee does. It
// returns -1 when no unit made so far can be col.
func (w *filtWalk) spendPip(t avail, st filtPos, sl Slot, fee, col int, held [kop.NumManaKinds]uint8) int {
	return w.spendPipMask(t, st, sl, fee, uint8(1)<<uint(col), held)
}

// spendPipMask is spendPip over a colour mask: each of its colours may pay
// the pip (a hybrid fee's two faces, CR 107.4c), so a group that can make any
// of them may fund it.
func (w *filtWalk) spendPipMask(t avail, st filtPos, sl Slot, fee int, pip uint8, held [kop.NumManaKinds]uint8) int {
	best := -1
	for g := 0; g < t.n; g++ {
		if t.cnt[g] == 0 || t.mask[g]&pip == 0 {
			continue
		}
		u := t
		if t.pain.on || t.nOne != 0 {
			u.total -= fee // the total-only reading, as the generic fee takes (the group and entry move together)
			return w.activated(u, st, sl, held)
		}
		u.removeWide(u.mask[g], 1)
		if fee > 1 {
			best = max(best, w.spendFee(u, st, sl, fee-1, 0, held))
		} else {
			best = max(best, w.activated(u, st, sl, held))
		}
	}
	return best
}

// spendFee takes owed more units of the filter's fee out of t's groups, from
// group index from on (a multiset of groups), then goes on with the walk.
func (w *filtWalk) spendFee(t avail, st filtPos, sl Slot, owed, from int, held [kop.NumManaKinds]uint8) int {
	if owed == 0 {
		return w.activated(t, st, sl, held)
	}
	best, found := -1, false
	for g := from; g < t.n; g++ {
		if t.cnt[g] == 0 {
			continue
		}
		u := t
		u.removeWide(u.mask[g], 1)
		best, found = max(best, w.spendFee(u, st, sl, owed-1, g, held)), true
	}
	if !found {
		t.total -= owed // no group can pay it: the total-only reading
		return w.activated(t, st, sl, held)
	}
	return best
}

// activated adds the filter sl's own mana (its fee already paid) and goes on.
func (w *filtWalk) activated(t avail, st filtPos, sl Slot, held [kop.NumManaKinds]uint8) int {
	for c, n := range held {
		if n != 0 {
			t.addUnits(uint8(1)<<c, n)
		}
	}
	w.s.filtProduce(w.k, &t, sl)
	return w.activate(t, st)
}

// windowFiltUseful: sl's filter can make a colour in want now, its fee
// covered by p's floating mana (gorge's paymentSourceUseful reads the window
// abilities, and a filter's is listed only once the pool pays it).
func (s *State) windowFiltUseful(k *prep, p uint8, sl Slot, want uint8) bool {
	if k.gen == nil || !k.gen.has(s.cardAt(sl), gfFilt) {
		return false
	}
	a := s.filtAb(k, sl)
	if a.AFlags&kop.AfManaDynamic != 0 && s.dynManaN(k, a, sl) <= 0 {
		return false // a zero evaluated amount makes no colour (CR 107.4a; payment_plan.go skips it)
	}
	if uint8(a.ManaColors)&want == 0 {
		return false
	}
	if col, ok := s.filtPip(k, sl); ok {
		return s.filtGateOn(k, sl) && s.pl[SeatIx(p)].pool[col] >= 1
	}
	if col, ok := s.filtRepeatPip(k, sl); ok && s.pl[SeatIx(p)].pool[col] < 1 {
		return false
	}
	if a, b, ok := s.filtHyb(k, sl); ok {
		pl := &s.pl[SeatIx(p)]
		return pl.pool[a] >= 1 || pl.pool[b] >= 1 // the plain pool, as the pip arm above reads it
	}
	return poolTotal(&s.pl[SeatIx(p)]) >= s.filtFee(k, sl)
}

// windowFiltOwedPick: the clamp kept sl (windowClampSkip read no colour of the
// cost still owed in its plain tap) only for a filter beside a plain ability,
// whose colours windowFiltUseful makes. This is the window whose ask carries
// the ManaPayment (xWindowPay, the announced X): gorge's owedAbilitiesFirst
// lists the owed-colour abilities first (a stable partition of the source's
// ability order), so the answer takes the filter, whose colour ask answers
// what the cost still owes (windowFiltFirst) -- the payWin window's
// payWinHybFilt arm, which lists the filter first for its lowest symbol. A
// window whose ask carries no ManaPayment keeps the listing order: gorge
// lists the plain tap first and takes it.
func (s *State) windowFiltOwedPick(k *prep, p uint8, sl Slot, useful *kop.ManaCost) bool {
	if k.gen == nil || !k.gen.has(s.cardAt(sl), gfFilt) || s.filtFirstOnly(k, sl) || !s.filtUsable(k, sl) {
		return false
	}
	want, decided := s.windowUseful(k, p, useful)
	if !decided {
		return false
	}
	if (s.windowSrcMask(k, sl)|s.windowEachMask(k, sl))&want != 0 {
		return false // the plain tap answers an owed colour first: the stable partition keeps it
	}
	r := s.windowFiltUseful(k, p, sl, want)
	return r
}

// windowRestrUseful: sl has a restricted tap mana ability that makes a colour in
// want. CR 106.6 limits what the mana may be spent on, not the activation, so the
// source stays in the window whatever is being paid for; gorge's paymentSourceUseful
// reads every window ability, and its answer then takes the first, plain one.
func (s *State) windowRestrUseful(k *prep, sl Slot, want uint8) bool {
	if k.gen == nil || !k.gen.anyRestr && !k.gen.anyRestrSrc && !k.gen.anyAnyR {
		return false
	}
	cd := &k.t.Cards[s.cardAt(sl)]
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind == kop.AbMana && a.ManaRestrict != 0 && a.Cost.Flags == kop.CostTap && uint8(a.ManaColors)&want != 0 {
			return true
		}
	}
	return false
}

// windowRestrOwedTap: a cast's ability ask lists sl's restricted ability first when
// its colour is owed and the plain one's isn't (gorge owedAbilitiesFirst, which reads
// colours only, so the mana may be mana this spell can't spend). It taps sl for that
// mana, floating beside the pool, and reports whether it did.
//
// A restricted any-colour ability is owed too: gorge's owedAbilitiesFirst adds every
// colour to a RestrictValid$ any-colour ability the payment admits, so the ask takes
// it ahead of a fixed {C} the payment cannot spend, and its colour ask then answers
// the pip (a hybrid face: Professor Zei's {U/R}{U/R} paid by Jasmine Dragon Tea
// Shop's Ally-only mana). Its owed colour is the lowest the payment still lacks.
func (s *State) windowRestrOwedTap(k *prep, p uint8, sl Slot, useful *kop.ManaCost, noOwed bool) bool {
	a, want, ok := s.windowRestrOwed(k, p, sl, useful)
	if !ok {
		return false
	}
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	cols := uint8(a.ManaColors) & want
	if _, decided := s.windowUseful(k, p, useful); noOwed && !decided {
		// an unsettled generic remainder leaves the colour ask unclamped, and a
		// window with no ManaPayment answers its lowest colour (windowCol)
		cols = uint8(a.ManaColors)
	}
	if !s.addRestrSrcV(k, p, sl, a.ManaRestrict, k.gen.restrVarOf(a), bits.TrailingZeros8(cols), max(a.ManaN, 1)) {
		s.capFull(StallRestrSrc)
	}
	return true
}

// windowRestrOwed reports the restricted ability of sl the cast's window
// takes ahead of sl's census one, and the colour want it floats. It is pure
// (no tap, no mana), so windowDry can ask it before simulating sl as an
// ordinary census source: the real walk taps the restricted ability instead,
// and the dry run's plain pool cannot model the restricted mana it floats.
func (s *State) windowRestrOwed(k *prep, p uint8, sl Slot, useful *kop.ManaCost) (ab *kop.Ability, want uint8, ok bool) {
	if k.gen == nil || !k.gen.anyRestrSrc {
		return nil, 0, false
	}
	w, decided := s.windowUseful(k, p, useful)
	if !decided {
		// gorge's owedAbilitiesFirst sorts an ability ahead when the colours a
		// fixed pip still owes are non-empty, whether or not the generic
		// remainder is settled (paymentUsefulColours' second result is not read).
		// windowDemand reads the fixed pips beside a hybrid pip, which leaves
		// windowUseful's own verdict undecided.
		w = s.windowDemand(k, p, useful)
	}
	if ra, ok := s.grantRestrOwnFirst(k, sl, w); ok {
		return ra, uint8(ra.ManaColors) & 0x3f, true // (mana_grantrestrictedown.go)
	}
	if w == 0 || s.windowSrcMask(k, sl)&w != 0 {
		return nil, 0, false
	}
	cd := &k.t.Cards[s.cardAt(sl)]
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		// An any-colour restricted ability (White Lotus Hideout's
		// Lesson/Shrine mana) is a choice-shaped Produced$ Any: gorge's
		// owedAbilitiesFirst lists it first when the payment owes a colour
		// its plain ability cannot make, and the answer takes its colour
		// ask. Tap it for the owed colour, which floats restricted
		// (CR 106.6), exactly as a fixed restricted one above. The batch
		// keeps the source's chosen creature type like every other tap:
		// a spell of the chosen type (an evoked Elemental Incarnation paid
		// with Eclipsed Realms' Elemental-restricted {B}) spends it only
		// when the batch records the type its restriction reads.
		anyColour := a.ManaColors == kop.ColAny
		if !restrSrcAb(a) || a.AFlags&kop.AfManaDynamic != 0 ||
			a.AFlags&kop.AfManaChoice != 0 && !anyColour || uint8(a.ManaColors)&w == 0 {
			continue
		}
		if anyColour {
			// gorge reads a RestrictValid$ any-colour ability's colours only
			// when the restriction admits THIS payment (owedAbilitiesFirst:
			// pay.RestrictValidMatches); otherwise it stays one manual option
			// with no enumerable colours, the fixed ability answers first, and
			// its mana is spent as that ability makes it (Jasmine's {C} paying
			// the generic of a Lesson the Ally-only mana cannot fund). The
			// fixed ones keep the window's printed offer.
			d := s.payDescNow(NoObj)
			if !s.restrSrcAdmits(k, p, sl, a, &d) {
				continue
			}
		}
		return a, w, true
	}
	return nil, 0, false
}

// censusGapMana: mana ability a is one gorge's planner census misses: a
// spend restriction, each-colour production, a cost beyond {T} (but
// {T} plus sacrificing itself, a Treasure's, and sacrificing itself alone,
// an Eldrazi Scion's, are the census's last-resort tier: gorge
// pay.PlanLastResortCostOK), or a
// deferred TriggersWhenSpent$ source (Pyromancer's Goggles; mana_deferred.go),
// or an unrestricted Combo Any one (Axebane Guardian; mana_comboany.go), or a
// reflected Amount$ one (Incubation Druid; mana_reflamt.go, the planner
// defers the amount, pay.PaymentPlanReflectedShape),
// so a filter beside it is no filter-only board (filtOnly). A sacrifice-only
// source (an Eldrazi Scion's "Sacrifice this creature: Add {C}") is no gap: the
// planner prices it as its source's last-resort tier (mana_sacsource.go).
// Neither is a restricted planner unit (restrSrcAb: Giada's angel-only {W},
// Untaidake's "{T}, Pay 2 life: Add {C}{C}", spend only on legendary spells):
// the census prices it per payment, its restriction admitting (mana_restrictedsource.go
// prepRestrSrc, mana_restrictedcost.go), so gorge's search activates no prefix
// for it and a filter beside it stays filter-only — Shadowblood Ridge's
// "{1},{T}: Add {B}{R}" is p1's fifth mana while it taps (CR 601.2g-h).
func censusGapMana(a *kop.Ability) bool {
	if a.AFlags&kop.AfLatent != 0 || restrSrcAb(a) {
		// a restricted planner unit: the census prices it, per payment
		return false
	}
	f := a.Cost.Flags
	return reflAmtManaAb(a) || comboAnyMana(a) || a.ManaDeferred || a.ManaRestrict != 0 || a.AFlags&kop.AfManaEachColor != 0 || f != kop.CostTap && f != kop.CostTap|kop.CostSacSelf && f != kop.CostSacSelf ||
		a.Cost.Mana.MV() != 0 || a.Cost.Mana.X != 0
}

// sacOnlyManaAb: mana ability a costs only sacrificing its source (an Eldrazi
// Scion token's "Sacrifice this creature: Add {C}", CR 605.1a): the census
// prices it as a last-resort unit, tapped or untapped alike (mana_sacsource.go,
// pcard.manaSacOnly's shape), so it is no census gap and a filter beside it is
// a filter-only board (prep's gfGapOther registration skips it). A deferral,
// a spend restriction or an each-colour production is another gap trigger's
// (censusGapMana's other disjuncts), and Lion's Eye Diamond's discard part is
// the DH gap's (gfDiscHand): those stay gaps.
func sacOnlyManaAb(a *kop.Ability) bool {
	if a.Cost.Flags != kop.CostSacSelf || a.Cost.Mana.MV() != 0 || a.Cost.Mana.X != 0 || a.Cost.DiscardHand {
		return false
	}
	return !(a.ManaDeferred || a.ManaRestrict != 0 && !restrSrcAb(a) || a.AFlags&kop.AfManaEachColor != 0 || comboAnyMana(a))
}

// registersGapOther: mana ability j of c (a) is a census gap of its own, for
// gfGapOther. Not Quirion Elves' chosen {T}, which prepMultiMana folds into the
// source's unit (registering it as a gap would keep a filter beside it off the
// filtScript path), not a sacrifice-only one, a census unit in the
// last-resort tier (a filter beside it stays filter-only), not a restricted
// "{T}, Pay N life" one, a restricted planner unit like the sacrifice-only
// source (restrLifeAb, mana_restrictedcost.go: gorge plans it as
// TierLastResort with the life a consequence, so a filter beside it stays
// filter-only), and not a sole
// life-only one, the planner's last-resort Consequence{Life} alternative the
// pain unit prepSoleLifeMana classifies (mana_pain.go).
func registersGapOther(c *kop.CardDef, a *kop.Ability, j int) bool {
	return censusGapMana(a) && !lifePainLandAb(c, a) && !kop.ChosenSecondManaAb(c, firstManaAb(c), j) && !sacOnlyManaAb(a) &&
		!restrLifeAb(a) && !kop.SoleGapLifeManaAb(c, j)
}

// filtOnly: p's census gaps are filters alone (no other activatable source
// with a mana ability the census misses: a Bloom Tender, a Courtyard, or a
// deferred TriggersWhenSpent$ source such as Pyromancer's Goggles, which
// otherGaps does not fund; mana_deferred.go). A restricted planner unit
// (Giada, Lupinflower Village's restricted {W}) is no census gap: the census
// prices it per payment, its restriction admitting (censusGapMana), so a
// filter beside it is still filter-only.
func (s *State) filtOnly(k *prep, p uint8) bool {
	return s.scriptFilt(k, p).any() && !s.gapOthers(k, p).andNot(s.restrGapOthers(k, p)).any()
}

// restrGapOthers: p's gapOthers whose mana is restricted per payment
// (Untaidake, the Cloud Keeper's "{T}, Pay 2 life: Add {C}{C}. Spend this
// mana only to cast legendary spells"). gorge's census misses the mana and
// its relaxed census admits it only for the payments its restriction
// matches (potential_plan.go paymentPlanRelaxedAlternatives,
// pay/plancost.go RestrictValidMatches), so it can fund neither a filter's
// fee nor a play the filter gap walk lists, and it keeps no filter off the
// walk the way an unrestricted gap's funding would (mana_mixgaps.go
// otherGaps). A life gap's mana is unrestricted (kop.GapLifeManaAb), so a
// life gap still counts: the mix search's otherLife step funds the filter
// beside it (mana_mixgaps.go).
func (s *State) restrGapOthers(k *prep, p uint8) slotMask {
	var m slotMask
	for b := s.gapOthers(k, p); b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		if k.gen.gapOther(c) >= 0 && k.t.Cards[c].Abilities[k.gen.gapOther(c)].ManaRestrict != 0 {
			m = m.with(sl)
		}
	}
	return m
}

// gapOthers: p's activatable sources with a census gap that is no filter. A tap-another
// source short of what its cost taps (Baylen with one token) cannot be activated, so
// gorge's census does not miss it. Neither does an ability that makes nothing
// (gorge's manaAbilityYieldsNothing: an Amount$ that resolves to zero): it can
// contribute to no payment, so no prefix activates it and it keeps no filter off the
// filter search. A deferred TriggersWhenSpent$ source
// (Pyromancer's Goggles) carries gfGapOther too (gen.go): otherGaps does
// not fund it (mana_deferred.go).
func (s *State) gapOthers(k *prep, p uint8) slotMask {
	m := s.gapMask(k, p, gfGapOther).or(s.lifeGapMask(k, p))
	if k.gen.anyDynGap {
		for b := m; b.any(); b = b.rest() {
			sl := b.first()
			c := s.cardAt(sl)
			a := &k.t.Cards[c].Abilities[k.gen.gapOther(c)]
			if a.AFlags&kop.AfManaDynamic != 0 && s.dynManaN(k, a, sl) == 0 {
				m = m.without(sl) // the amount is zero: the ability yields nothing
			}
		}
	}
	if k.sacOnlyMana {
		// A sacrifice-only source (an Eldrazi Spawn or Scion token's "Sacrifice
		// this creature: Add {C}") is a payment-plan last resort, not a census
		// gap (mana_sacsource.go): gorge's planner prices it inside every
		// plan, so a filter beside it is still a filter-only board.
		for b := m; b.any(); b = b.rest() {
			if sl := b.first(); s.manaPCAt(k, sl).manaSacOnly {
				m = m.without(sl)
			}
		}
	}
	if k.ctrMana && m.any() {
		// A counter-cost source with too few counters to remove (Ramos,
		// Dragon Engine with one +1/+1 counter) cannot be activated either:
		// gorge's census lists no ability of it, so a filter beside it is
		// still a filter-only board.
		live := s.scriptCtr(k, p)
		for b := m; b.any(); b = b.rest() {
			sl := b.first()
			c := s.cardAt(sl)
			if k.ctrAbOf(c) != nil && int(k.ctrAb[c]) == int(k.gen.gapOther(c)) && !live.has(sl) {
				m = m.without(sl)
			}
		}
	}
	if !k.tapMana {
		return m
	}
	live := s.scriptTapX(k, p)
	for b := m; b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		if k.tapXAbOf(c) != nil && !live.has(sl) {
			m = m.without(sl)
		}
	}
	return m
}

// filtStep is one activation of a filter script: source sl; for a gap
// (a filter source) ans 0 is its free ability and 1+c the filter making
// colour c; for a funding source ans is the colour it makes.
type filtStep struct {
	sl  Slot
	ans uint8
	gap bool
	// alloc: a combination filter's production (Baxter Building: four mana
	// in any combination), one count per W U B R G slot. It replaces ans's
	// single colour; the step is still a gap activation (ans > 0).
	alloc [kop.NumManaKinds]uint8
	// restr: a funding source's answer is its restricted ability's mana
	// (ans its colour): gorge's activation ask lists the restricted
	// ability beside the census one, and the search reads no restriction
	// (mana_restrictedsource.go)
	restr bool
	// sacOnly: a funding source whose mana ability costs only its own
	// sacrifice (an Eldrazi Spawn): it adds its mana, is not tapped, and ceases
	// (mana_sacsource.go)
	sacOnly bool
	// wide: a funding source's answer is a gate-on pair's larger ability (the
	// second of Whisperer of the Wilds' two: gorge's ability ask lists both
	// and the search takes each in turn; mana_gate.go gatedLargerPair), its
	// mana srcYield's, where the census one makes manaN
	wide bool
	// hand: a funding step that exiles the hand guide at index hidx of the
	// payer's hand (a Spirit Guide's hand mana ability, mana_handguide.go);
	// sl is NoSlot
	hand bool
	hidx uint8
	// other: a census gap that is no filter (mixed gaps, zmixgaps file), or
	// an answer or a pass while a step's trigger resolves (act)
	other otherStep
	act   Action
}

type filtKey struct {
	pool [kop.NumManaKinds]uint8
	// art: the pool slots holding an artifact's mana (genState.poolArt):
	// gorge's key reads the typed units apart (Player.ManaUnits), so a
	// Three Tree Mascot's {W} over a Plains' {W} is a new state
	art    uint8
	typed  [kop.NumManaKinds]uint8 // the artifact-typed units by colour (the slots of art, a typed restricted batch)
	tapped slotMask
	// bf: the payer's permanents. gorge's key lists every own battlefield
	// object (scriptStateKey), so a sacrificed token, which ceases to exist
	// and leaves no graveyard card, is a new state beside the one it was
	// sacrificed from: an Eldrazi Spawn's {C} spent into a filter's fee leaves
	// the pool and every tapped bit as they were.
	bf slotMask
	// ctr: the counters of the counter-cost gaps (Vivid Meadow's charge
	// counters): gorge's key reads every own permanent's counters
	// (scriptStateKey), so Meadow's {W} made by removing a counter is a new
	// state beside its plain {W} (mana_filterctrkey.go)
	ctr  [8]uint16
	sp   uint8 // gorge's key counts the stack, hand and graveyard too (a Taps trigger's loot)
	hand uint8
	gy   uint8
}

// maxFiltPath bounds one searched path. gorge's scriptSearch has no path
// length of its own, only the potentialScriptNodes it visits, and every path
// step is one of those nodes, so a path longer than the budget cannot be
// reached. Sixteen was smaller: a repeatable fee-only filter funds its next
// activation from the pool, so the search walks chains longer than sixteen
// steps (Simic Cluestone's {G}{U} through repeated Signpost Scarecrow
// activations) and the cap dropped the branch gorge takes.
const maxFiltPath = potentialScriptNodes

// maxFaceFilters bounds the filters faceReachable tries in combination.
const maxFaceFilters = 6

// filtSearch is gorge's prefix search (scriptSearch.visit) over filter gaps.
type filtSearch struct {
	a     Action
	p     uint8
	gaps  slotMask // the root census's filter gaps
	other slotMask // its gaps that are no filter (otherGap)
	// dead: the gaps gapSpendableMask dropped. Neither gaps nor funding
	// sources: their mana is restricted away from the play, and a funding
	// tap of one floats only that restricted mana, which pays no fee the
	// restriction refuses (CR 106.6), as the grant search's dropped holders
	// fund nothing (grantSrch.candidates).
	dead  slotMask
	fees  int   // the funding the search may activate
	fundN int   // the untapped non-gap sources that can fund that fee
	own   Slot  // the source whose {T} is the play's own cost (NoSlot: none)
	sp0   uint8 // the stack at the root: only a deeper one is the path's to resolve
	nodes int
	nseen int
	// seen is the play search's visited set, potentialScriptNodes long. A pay
	// search (filtPay) browses the same nodes under its own, wider budget, so
	// it keeps its table in paySeen and this one stays the size its search can
	// fill (seenTable).
	seen    [potentialScriptNodes]filtKey
	paySeen []filtKey
	n       int
	path    [maxFiltPath]filtStep
	// spill: a step of the path found floated restricted mana past what
	// State.rb holds (a restriction past restrKinds), unrestricted: the
	// script the search found is not one gorge's planner would price
	spill uint8
	// stall: the first stall a failed line's fork latched (stallOf), StallNone
	// none; filtScript declares it when the whole search fails
	stall uint8
	// need is the colours the play's printed base announces (playNeed, the
	// set the grant search reads too): the filter's colour ask tries one of
	// those first, the mirror of gorge's scriptOrderAnswers on the same ask.
	// It reorders the answers, never the answer set.
	need [kop.NumManaKinds]bool
	// lifeUsed: the life-only sources already activated on this path. A life
	// cost is no {T}, so the source is repeatable, but gorge's prefix search
	// (scriptSearch.visit) walks its gap list once: each gap activates at most
	// once per path.
	lifeUsed slotMask
	// useful is the play's printed mana cost (filtUsefulCost), the fixed
	// pips the fee-stall clamp (filtFeeColStalls) reads; usefulOK false (a
	// play whose printed cost the search does not read: a kicked, bargained
	// or flashback cast, an {X}, snow, hybrid, twobrid or Phyrexian pip) and
	// the clamp fails open, the branch explored (gorge's scriptSearch.pipOK,
	// engine/rules/potential_script.go).
	useful   kop.ManaCost
	usefulOK bool
	// feeOnly: the search's gaps hold a fee-only repeatable filter (gorge's
	// scriptSearch.feeOnly). The activation-site and one-shot cuts are scoped
	// to it, so a board with no such source keeps the base node walk other
	// searches share a budget with (the granted-mana prefix). The twin cut
	// runs on this fee-only path and for a printed or gained activated play
	// (isAbilityPlay): two indistinguishable sources reach the same payer
	// states, and the skip is what prices a fee-carrying gap beside a row of
	// identical basics (tail-26 pair 26). An ordinary cast and every 0x80
	// grant keep their older walk.
	feeOnly bool
	// fundCap: a fallback search's cap on the covered funding taps (0: the
	// census's fees). The plain search's node walk explores funding
	// permutations depth first; a fixed-cost activation whose payment needs
	// only a few sources can exhaust the budget in deeper branches before it
	// reaches one, so filtScript reruns the search capped at the play's
	// mana value and then one step at a time (filtScriptRes's staged fallback).
	fundCap int
	// hands: the hand guides the path exiled so far (mana_handguide.go)
	hands uint8
	// pay: a payment search's cost (filtPay): the terminal is the pool
	// paying it, not a play being offered. A pay search of the unless / ward
	// census (census) skips the hand guides unlessWindowUnits prices no unit
	// and its gap set is filtered to the census units at the root.
	pay    *kop.ManaCost
	census bool
	// phased: gorge's fallback run (potentialPrefixScript's second build, the
	// phased order of scriptSearch.activations): the gaps whose ability costs
	// no mana, then the covered funding sources, then the gaps that carry a
	// fee (paid from the floating pool, CR 601.2b), and without the plain
	// run's fee-stall prune (feeStall is set on the plain run only).
	phased bool
	// feeOrder: the phased run's generic mana-fee order (gorge's
	// scriptSearch.feeOrder): the covered funding first, then the gaps that
	// carry a multi-unit fee (feeGaps), then the other gaps (mana_filterfeeorder.go).
	feeOrder bool
	feeGaps  slotMask
}

// deferGapLeft: the search still holds an untapped filter gap whose tap
// queues a Taps trigger (a land under Manabarbs). That tap is gorge's
// window's colour-asked tap: it must resolve at its ask, before the ability
// is pushed, so its trigger lands below the ability it paid for. A filter
// whose tap queues no trigger keeps the offer check (the planner pays it
// after the push, as a cast's does), and a node with no such gap left has
// spent the taps that matter.
func (fs *filtSearch) deferGapLeft(k *prep, t *State) bool {
	for m := fs.gaps; m.any(); m = m.rest() {
		sl := m.first()
		if t.perms[sl].flags&pfTapped != 0 || sl == fs.own || fs.dead.has(sl) {
			continue
		}
		if t.tapsDeferred(k, fs.p, sl) {
			return true
		}
	}
	return false
}

// poolPaysPlay: the floating pool pays activation a's reduced mana cost, the
// point a filter prefix's payment window has tapped everything it needs. A
// cost the walk cannot read, or one with an announced {X}, keeps the offer
// check: the search cannot price it and must not keep tapping.
func (s *State) poolPaysPlay(k *prep, p uint8, a Action) bool {
	if isGainedMode(a.Mode()) || a.Mode()&0x80 != 0 {
		return true // a gained or granted ability: no own index to price
	}
	m, _, ok := s.activationMana(k, Obj(a.Source()), a.Mode())
	if !ok || m.X != 0 || m.XPip != 0 {
		return true
	}
	return s.poolPays(p, &m)
}

// filtFunded reports whether gorge's census funds the fees of the filter gaps f:
// an unrelaxable census (or a live tap-X pair, or a live no-{T} sacrifice
// source: Skirk Prospector's "Sacrifice a Goblin: Add {R}", whose cost is not
// once-only) pays none, so no search funds one.
// A live storage battery (Subterranean Hangar's "{T}, Remove X storage
// counters: Add {B} for each") makes the census unrelaxable: its announced X
// has no once-only relaxed alternative (paymentPlanRelaxedAlternatives), so a
// School of the Unseen beside it is funded by nothing (potential_plan.go
// paymentPlanCensus.fees).
func (s *State) filtFunded(k *prep, p uint8, f slotMask) bool {
	return s.filtRelaxable(k, f) && s.filtFundable(k, p, f)
}

// filtFundable is filtFunded for a prefix search's funding: a hybrid-fee
// filter is left out of the unit bounds and the relaxed-subset count (gorge
// cannot relax its pip: a hybrid fee has no relaxed alternative), but its fee
// is funded like any other (mana_filterhybrid.go): its activation pays the pip
// from floating mana, so the search must tap a covered source first. Beside a
// first-and-only filter (Apprentice Wizard's "{U}, {T}: Add {C}{C}{C}") the
// wizard's fee is still fundable and the search must not give up on it
// (tail-66-c000 pair 4: Arcane Melee).
func (s *State) filtFundable(k *prep, p uint8, f slotMask) bool {
	n := 0
	for ; f.any(); f = f.rest() {
		sl := f.first()
		if _, _, hyb := s.filtHyb(k, sl); hyb {
			// A hybrid fee has no relaxed alternative, so it adds no subset
			// to the relaxed proof and does not count toward that bound;
			// each one still funds its own fee (filtFunding).
			continue
		}
		if n++; n > relaxedPaidLimit {
			return false
		}
		if !s.filtUnitsEnumerable(k, sl) {
			return false
		}
	}
	return !(k.tapMana && (s.tapXTwoLive(k, p) || s.tapXSacAnyLive(k, p))) && !s.storeGapLive(k, p)
}

// storeGapLive reports whether p controls an activatable lone storage
// battery: a store counter-cost gap holding counters to remove (scriptCtr)
// whose card has no plain first mana ability (ctrSecond). A later battery
// (Mage-Ring Network's {T}: Add {C} beside its counter-X ability) is priced by
// that first unit, so it leaves the census relaxable and the filter's fee
// fundable, as storeCensusGap reads the same ctrSecond test.
func (s *State) storeGapLive(k *prep, p uint8) bool {
	for b := s.scriptCtr(k, p); b.any(); b = b.rest() {
		c := s.cardAt(b.first())
		if kop.StoreManaAb(k.ctrAbOf(c)) && !k.ctrSecond(c) {
			return true
		}
	}
	return false
}

// phasedFallback: the plain run found no script after spending its whole node
// budget on a board whose gaps are filters alone, one of them a repeatable
// fee-only one -- the case gorge's phased run takes outside its own scope
// (scriptSearch.plainLimited). Boards with another kind of gap keep the plain
// run's answer.
func (fs *filtSearch) phasedFallback() bool {
	return fs.feeOnly && !fs.other.any() && fs.pay == nil && fs.nodes >= potentialScriptNodes
}

// filtFunders counts p's untapped battlefield sources that are no gap and
// can fund a filter's fee: the funding pass's own sources.
func (s *State) filtFunders(k *prep, p uint8, gap slotMask) int {
	n := 0
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		if gap.has(sl) || s.perms[sl].flags&pfTapped != 0 {
			continue
		}
		if s.canTapForMana(k, sl) || s.fundsBySac(k, sl) {
			n++
		}
	}
	return n
}

// filtScript reports whether gorge's prefix search finds a script for play
// a of p; with apply it plays the script on s (the play's prefix).
//
// On the autopay surface, when the search spends its whole 160-node budget on
// an activation, a staged fallback reruns it with a funding cap at the play's
// mana value and
// then one step at a time: a fixed cost needs at most its mana value's taps
// from the covered sources plus the filters' fees, and the capped walk reaches
// that funding line before the uncapped depth-first walk's dead permutations
// spend the budget (gorge's planner still offers the play on a search_limit
// plan, legal_walk_autopay.go autoPayPlan).
//
//go:noinline
func (s *State) filtScript(k *prep, a Action, p uint8, apply bool) bool {
	memo := s.filtMemoLookup(k, a, p, apply)
	if memo.hit && !DebugFiltMemoCheck {
		return false // this game already failed this play's search in this transition (mana_filtmemo.go)
	}
	var fs filtSearch
	fs.a, fs.p, fs.gaps, fs.own, fs.sp0 = a, p, s.scriptFilt(k, p), NoSlot, s.sp
	fs.need = s.playNeed(k, a) // the play's announced colours, whichever search prices it
	fs.useful, fs.usefulOK = s.filtUsefulCost(k, a)
	fs.other = s.otherGaps(k, p, a)
	fs.feeOnly = s.filtRepeatIn(k, fs.gaps)
	if a.Kind() == AkActivate {
		_, fs.own, _ = s.activationMana(k, Obj(a.Source()), a.Mode())
	}
	fs.fees = s.filtFunding(k, p, fs.gaps)
	fs.fundN = s.filtFunders(k, p, fs.gaps.or(fs.other))
	hadOther := fs.other.any() // a dropped gap still asks for the dead-end check
	if gapFilteredPlay(a) && (fs.gaps.any() || fs.other.any()) {
		d := s.grantPayDesc(k, a)
		g0, o0 := fs.gaps, fs.other
		fs.gaps, fs.other = s.gapSpendableMask(k, fs.gaps, &d), s.gapSpendableMask(k, fs.other, &d)
		fs.dead = g0.andNot(fs.gaps).or(o0.andNot(fs.other))
		if !fs.gaps.any() && !fs.other.any() {
			return false
		}
	}
	t := s.fork()
	defer s.unfork()
	t.g.noGrant = a.Kind() == AkActivate // the activation's granted mana is the grant search's
	t.g.fbProbe = s.fbProbeRoot(k, p, a)
	s.pool.inFilt++ // (fork made the pool)
	found := fs.visit(k, t, 0, false)
	nodes := fs.nodes
	if !found && s.feeOrderFallback(k, p, a, &fs) {
		// gorge's plain run spent its node budget walking the filters' fee
		// branches before the fee was funded: its fee-order run floats the
		// covered funding first (mana_filterfeeorder.go)
		ph := filtSearch{a: fs.a, p: fs.p, gaps: fs.gaps, own: fs.own, sp0: fs.sp0, need: fs.need, useful: fs.useful,
			usefulOK: fs.usefulOK, other: fs.other, feeOnly: fs.feeOnly, fees: fs.fees, dead: fs.dead, phased: true,
			feeOrder: true, feeGaps: fs.feeGaps}
		if found = ph.visit(k, t, 0, false); found {
			fs = ph
			nodes = ph.nodes
		} else {
			nodes = max(nodes, ph.nodes)
			if fs.stall == 0 {
				fs.stall = ph.stall
			}
		}
	} else if !found && fs.phasedFallback() {
		// gorge's plain run spent its node budget on a repeatable fee-only
		// filter's branches: its phased run walks the funding line the plain
		// order walked past (potentialPrefixScript)
		ph := filtSearch{a: fs.a, p: fs.p, gaps: fs.gaps, own: fs.own, sp0: fs.sp0, need: fs.need, useful: fs.useful,
			usefulOK: fs.usefulOK, other: fs.other, feeOnly: fs.feeOnly, fees: fs.fees, dead: fs.dead, phased: true}
		if found = ph.visit(k, t, 0, false); found {
			fs = ph
			nodes = ph.nodes
		} else {
			nodes = max(nodes, ph.nodes)
			if fs.stall == 0 {
				fs.stall = ph.stall
			}
		}
	}
	if !found && s.cfg&cfgGorgeAutopay != 0 && nodes >= potentialScriptNodes && a.Kind() == AkActivate && fs.usefulOK && fs.useful.MV() > 0 {
		fees := 0
		for m := fs.gaps; m.any(); m = m.rest() {
			fees += s.filtFee(k, m.first())
		}
		base := fs.useful.MV()
		for cap := base; cap <= base+fees; cap++ {
			fb := filtSearch{a: fs.a, p: fs.p, gaps: fs.gaps, own: fs.own, sp0: fs.sp0, need: fs.need, useful: fs.useful,
				usefulOK: fs.usefulOK, other: fs.other, feeOnly: fs.feeOnly, fees: fs.fees, dead: fs.dead, fundCap: cap}
			if fb.visit(k, t, 0, false) {
				fs = fb
				found = true
				break
			}
		}
	}
	s.pool.inFilt--
	if !found {
		if fs.stall != 0 {
			// every line failed and one ran into a rule gap the kernel stalls by
			// name: declare it, not "no payment" (tapXScript's stall; a found
			// path is not this, its stall is fs.spill's below)
			s.capFull(fs.stall)
		} else {
			s.filtMemoNote(&memo)
		}
		return false
	}
	if memo.hit {
		panic("dirac: filtScript memo: a remembered failure found a script")
	}
	if fs.spill != 0 {
		s.capFull(fs.spill) // (the fork the step ran on latched it; the real state must)
	}
	if !apply && (k.restrSrc || hadOther) && s.filtDeadEnd(k, p, a, &fs) {
		return false
	}
	if apply {
		s.filtPlay(k, p, fs.path[:fs.n])
	}
	return true
}

// filtPay is the unless / ward window's affordability probe for a filter-first
// source's fee-funded chain: the same prefix search a cast's payment runs
// (filtScript), with the terminal "the pool pays" instead of "the play is
// offered". A filter's generic fee may need a funder the cost does not owe
// (CR 605.3b), and the funder may list before the filter (Overflowing Basin's
// {1}, {T}: Add {G}{U} beside Power Depot's {C}).
//
// The probe is the unless / ward census's, not a cast payment's: it prices
// only the sources gorge's unlessWindowUnits lists (censusPayGaps), skips the
// hand guides the census leaves out, and runs with the pool's census flag set
// so a Produced$ Chosen TapsForMana extra never joins a tap. takeDeferred is
// the census's: an unless / ward window takes a planner-deferred source once
// the rest cannot cover the cost. The second return is a FAILED search's first
// latched rule-gap stall (StallNone none), or a found census plan's own spill:
// a found plan never carries a failed sibling line's stall (filtScript's
// shape). A plain window's found spill stays zero: its five windowAffords
// callers read a found probe as final and would drop it, so the scope keeps
// their pre-found-spill answer (only the census probe is declared at its drop
// sites, windowPays' callers). The caller declares the return at its drop
// site, never the probe. s is not mutated.
//
// The live window's answer is not this plan: gorge's window taps in its
// answer order and may fail short where the census proves pay (Lantern of
// Revealing is asked its first colour, not the red pip still owed), after
// which the trigger takes the unpaid branch. The kernel's payWindow mirrors
// that answer, not the plan; executing the plan there would make dirac pay
// where the leader sacrificed and diverge (TestMBCUnlessEachManaFeePayAnswer).
func (s *State) filtPay(k *prep, p uint8, cost *kop.ManaCost, takeDeferred bool) (bool, uint8) {
	if k.gen == nil || !k.gen.anyFiltFirst || cost == nil {
		return false, 0
	}
	var fs filtSearch
	fs.a = MakeAction(AkPass, 0, 0, 0)
	fs.p, fs.gaps, fs.own, fs.sp0 = p, s.scriptFilt(k, p), NoSlot, s.sp
	fs.other = s.censusPayGaps(k, p, s.otherGaps(k, p, fs.a), takeDeferred)
	// A payment plan taps the cost's own sources beside the filters' fees, so
	// the funding bound is every slot rather than the fees summed (filtFunding
	// limits a play's prefix, whose ordinary sources the planner counts, not
	// taps). The terminal stops the search once the pool pays.
	fs.fees = MaxPerm
	fs.pay = cost
	fs.census = takeDeferred
	fs.useful, fs.usefulOK = *cost, true
	t := s.fork()
	defer s.unfork()
	s.pool.payCalls++
	var own bool
	fs.paySeen, own = s.pool.takePaySeen()
	if own {
		defer func() { s.pool.payBusy = false }()
	}
	was := s.pool.census
	s.pool.census = takeDeferred
	s.pool.inFilt++
	found := fs.visit(k, t, 0, false)
	s.pool.inFilt--
	s.pool.census = was
	return found, filtPayStall(&fs, found, t.g.stalled)
}

// filtPayStall is filtPay's second return: a FAILED search's first latched
// rule-gap stall (fs.stall, or the root fork's when the lines latched none),
// never a found plan's; a found census plan's own spill (filtScript's shape:
// fs.stall belongs to a failed line, fs.spill to the found one); and nothing
// for a plain window's found plan, whose five windowAffords callers read a
// found probe as final.
func filtPayStall(fs *filtSearch, found bool, walkStall uint8) uint8 {
	if !found {
		if fs.stall != 0 {
			return fs.stall
		}
		return walkStall
	}
	if fs.census {
		return fs.spill
	}
	return 0
}

// censusPayGaps is otherGaps as the unless / ward census prices it: the set
// of gaps filtPay's prefix search may activate. The census walk lists no
// tap-another source without a live window mode (windowPick's tapX arm), and
// without takeDeferred no planner-deferred source either; a bounded rider
// whose cost or production the census does not price (Gemstone Mine's any
// colour) is no unit there (riderCensusFree), so it is no unit here.
func (s *State) censusPayGaps(k *prep, p uint8, m slotMask, takeDeferred bool) slotMask {
	if k.gen == nil {
		return slotMask{}
	}
	if k.tapMana && s.g.winMode == winNone && !s.castTapX(k) {
		// windowPick lists a tap-another source only with a live window mode
		// or a cast's own tap-X build; otherwise it is no census unit.
		m = m.andNot(s.scriptTapX(k, p))
	}
	if !takeDeferred {
		// a plain window skips the planner-deferred sources, and takes the
		// other gaps (a rider) as its walk does.
		return m.andNot(s.scriptDeferred(k, p)).andNot(s.scriptTapsDefer(k, p))
	}
	for b := m; b.any(); b = b.rest() {
		sl := b.first()
		if a, ok := s.riderAbilityOf(k, sl); ok && !riderCensusFree(a) {
			m = m.without(sl)
		}
	}
	return m
}

// filtPlay plays a found path on s: each activation as the search made it,
// each answer and pass as the decision then pending (the other player passes
// while the trigger resolves, tapXPlay). Each tap fires its Taps triggers
// once, in filtDo (filtUse, otherDo), as the search's own steps did.
func (s *State) filtPlay(k *prep, p uint8, path []filtStep) {
	n := s.Decisions
	var d Decision
	for _, st := range path {
		if st.other.kind == otherAnswer || st.other.kind == otherPass {
			if s.tapXNext(p, &d) && hasOption(&d, st.act) {
				s.Apply(st.act)
			}
			continue
		}
		s.filtDo(k, p, st)
	}
	s.Decisions = n
}

// DebugFiltNodeBudget caps fs.tick's budget below the search's own,
// mutation testing only: a test pinning the search's node accounting shrinks
// the budget to the line it pins. Never above potentialScriptNodes for a play
// search (its seen set is that long); 0 is the search's own budget. A pay
// search's budget is paySearchNodes and its own table is that long, so the cap
// works there too.
var DebugFiltNodeBudget int

// DebugFiltTryGapExempt limits try's priced-node exemption from the visited-set
// cut to gap lines, as it was before the hand guides' funding pass shared it
// (mutation testing only: the Spirit Guide lockstep must catch it).
var DebugFiltTryGapExempt bool

// tick counts one visited node; false once the budget is spent.
func (fs *filtSearch) tick() bool {
	budget := potentialScriptNodes
	if fs.pay != nil {
		budget = paySearchNodes
	}
	if DebugFiltNodeBudget != 0 && DebugFiltNodeBudget < budget {
		budget = DebugFiltNodeBudget
	}
	if fs.nodes >= budget {
		return false
	}
	fs.nodes++
	return true
}

// filtDo plays step st for p.
func (s *State) filtDo(k *prep, p uint8, st filtStep) {
	if st.hand {
		s.handFundDo(k, p, st)
		return
	}
	if st.other.kind != otherNone {
		s.otherDo(k, p, st)
		return
	}
	s.fireFiltAbilityCast(k, p, st) // the activation precedes its cost's tap and sacrifice
	pl := &s.pl[SeatIx(p)]
	if st.sacOnly {
		s.fundSacOnly(k, p, st.sl) // (mana_filtfund.go)
		return
	}
	if st.gap && st.ans > 0 {
		s.filtUse(k, st.sl) // (a once-each-turn filter spends its use, not its {T})
	} else if st.gap || st.restr || !s.fundsBySac(k, st.sl) {
		s.perms[st.sl].flags |= pfTapped
		s.touchSl(st.sl)
		s.manaTapped(k, st.sl)
	}
	switch {
	case st.gap && st.ans > 0:
		// the fee is paid from the pool, generic in C W U B R G order, the
		// restricted mana the activation may spend in line with the plain
		// ones (filtFeeSpend); a one-pip filter's {c} is paid in c
		// (mana_filterpip.go)
		dc, dn := s.deferSnap(p) // a deferred source's unit the fee spends is gone (mana_deferred.go)
		fee := s.filtFee(k, st.sl)
		if col, ok := s.filtPip(k, st.sl); ok {
			s.filtPayPip(k, p, st.sl, col)
			fee = 0
		} else if col, ok := s.filtRepeatPip(k, st.sl); ok {
			s.filtPayPip(k, p, st.sl, col) // a fee-only filter's pip first, then its generic part
			fee--
		} else if a, b, ok := s.filtHyb(k, st.sl); ok {
			s.filtPayHyb(k, p, st.sl, a, b) // the hybrid pip, in its first printed colour when held
			fee = 0
		}
		fee = s.filtFeeSpend(k, p, st.sl, fee)
		if k.artMana {
			s.artSync(p) // a slot the fee emptied no longer holds an artifact's mana
		}
		s.deferSync(p, dc, dn)
		if s.filtAb(k, st.sl).ManaEach {
			s.filtEachDo(k, p, st.sl) // every colour at once, whatever ans names (mana_filtereach.go)
			break
		}
		if st.alloc != ([kop.NumManaKinds]uint8{}) {
			// a combination filter: its allocation, each colour registered
			// as the tap's (Baxter Building). It taps a land, so a live
			// tap-extra must stall like every other filter activation's.
			for c, n := range st.alloc {
				if n == 0 {
					continue
				}
				pl.pool[c] += n
				s.artTag(k, p, st.sl, c)
			}
			s.tapExtraStall(k, st.sl)
			break
		}
		s.poolAdd(p, int(st.ans-1), int(s.filtUnits(k, st.sl)))
		// the filter taps the source for mana, so a TapsForMana static adds
		// its extra beside the filter's unit (mana_tapextra.go filtExtra)
		xv := s.filtExtra(k, st.sl)
		s.addXv(p, &xv)
		s.artTag(k, p, st.sl, int(st.ans-1)) // an artifact's mana is typed (Three Tree Mascot, Prophetic Prism; mana_artifact.go)
		if s.filtAb(k, st.sl).Cost.Flags&kop.CostSacSelf != 0 {
			// {1}, {T}, sacrifice this (Omni-Cheese Pizza): the sacrifice is
			// part of the activation cost (CR 602.2b, 605.1a), taken after the
			// mana is added, as a Treasure's is (fundSac)
			s.sacFilterSource(k, st.sl)
		}
	case st.gap:
		pc := s.manaPCGen(k, st.sl)
		c := bits.TrailingZeros8(s.srcMaskAt(pc, st.sl))
		s.poolAdd(p, c, int(max(pc.manaN, 1)))
		s.addExtraPick(k, p, st.sl, pc, c)
		s.artTag(k, p, st.sl, c)
		s.tapPain(k, p, st.sl, c)
	case st.restr:
		s.filtRestrDo(k, p, st)
	default:
		n := int(max(s.manaPCGen(k, st.sl).manaN, 1))
		if st.wide {
			n = int(srcYield(s.manaPCGen(k, st.sl)))
		}
		s.poolAdd(p, int(st.ans), n)
		s.addExtraPick(k, p, st.sl, s.manaPCGen(k, st.sl), int(st.ans))
		s.artTag(k, p, st.sl, int(st.ans))
		s.tapPain(k, p, st.sl, int(st.ans)) // Cabal Pit's rider resolves with its mana (CR 605.3b)
		if k.sacMana {
			s.fundSac(k, st.sl)
		}
	}
}

// fireFiltAbilityCast queues AbilityCast listeners for the mana ability a
// filt path step is about to activate (CR 605.1a), resolved here from the step
// that activates it: a filter's own ability, a restricted source's recorded
// one, or the source's mana ability that makes the answered colour. A granted
// activation is skipped (fireViewAbilityCast).
func (s *State) fireFiltAbilityCast(k *prep, p uint8, st filtStep) {
	if k.gen == nil || !k.gen.anyAbilityCast || st.sl == NoSlot {
		return
	}
	c := s.cardAt(st.sl)
	switch {
	case st.gap && st.ans > 0:
		s.fireManaAbilityCast(k, c, k.gen.filt(c), p)
	case st.restr:
		s.fireManaAbilityCast(k, c, k.gen.restrRecOf(c).ab, p)
	case st.gap:
		s.fireViewAbilityCast(k, p, st.sl, s.manaPCGen(k, st.sl).manaAb)
	case st.wide:
		s.fireViewAbilityCast(k, p, st.sl, int8(s.widePairAb(k, st.sl, s.manaPCGen(k, st.sl))))
	default:
		s.fireViewAbilityCast(k, p, st.sl, s.manaAbForCol(k, st.sl, int(st.ans)))
	}
}

// manaAbForCol is the mana ability of the planner's view of sl that makes
// colour col: the view's own when it can, else the card's first mana ability
// that can, else the view's (a multi-ability source's answer names a colour,
// not an ability).
func (s *State) manaAbForCol(k *prep, sl Slot, col int) int8 {
	ab := s.manaPCGen(k, sl).manaAb
	cd := &k.t.Cards[s.cardAt(sl)]
	if ab >= 0 && int(ab) < len(cd.Abilities) && uint8(cd.Abilities[ab].ManaColors)&(1<<col) != 0 {
		return ab
	}
	for j := range cd.Abilities {
		if a := &cd.Abilities[j]; a.Kind == kop.AbMana && uint8(a.ManaColors)&(1<<col) != 0 {
			return int8(j)
		}
	}
	return ab
}

func poolTotal(pl *player) int {
	n := 0
	for _, v := range pl.pool {
		n += int(v)
	}
	return n
}

// try plays st on a copy of t and continues the search there.
func (fs *filtSearch) try(k *prep, t *State, st filtStep, funded int, gapUsed bool) bool {
	if fs.n == maxFiltPath {
		return false
	}
	c := t.fork()
	defer t.unfork()
	q := c.npend
	life := st.other.kind == otherLife
	if life {
		fs.lifeUsed = fs.lifeUsed.with(st.sl) // once per path (gorge's gap list)
	}
	lifeBefore := c.pl[SeatIx(fs.p)].life
	c.filtDo(k, fs.p, st)
	if lifeBefore > 0 && c.pl[SeatIx(fs.p)].life <= 0 {
		// the step's own pain killed the payer: gorge's scriptSearch.visit
		// ends the line at the game over (CR 104.3a, 704.5a), whatever the
		// pool then pays. The dead child is still a visited node of the
		// budget there (sr.nodes charges before the game-over check), so the
		// step costs its node: a search that walked lethal answers for free
		// reached lines gorge's budget spent on them first (Karplusan
		// Forest's painful {R} or {G} at one life)
		if life {
			fs.lifeUsed = fs.lifeUsed.without(st.sl)
		}
		fs.tick()
		return false
	}
	fs.path[fs.n] = st
	fs.n++
	if c.npend <= q {
		// The visited-state check is hoisted here (gorge's scriptSearch.try,
		// cut G): a duplicate payer state is not a node of the search. The
		// offered scan runs first, exactly as visit's does (the colour-only
		// key cannot tell a restricted unit from a filter's converted one),
		// so a priced node is never skipped. The budget is potentialScriptNodes
		// *distinct* states. A node is priced only once a gap or a hand guide
		// has made mana the raw priority list did not count (visit's scan), and
		// a guide's exile leaves the same key whichever guide it was (the
		// play's own card among them).
		if fs.feeOnly && !((gapUsed || fs.hands > 0 && !DebugFiltTryGapExempt) && fs.offers(k, c)) && fs.seenKey(k, c) {
			fs.n--
			if life {
				fs.lifeUsed = fs.lifeUsed.without(st.sl)
			}
			return false
		}
	}
	var ok bool
	if c.npend > q && !DebugFiltNoTrigSettle {
		ok = fs.settle(k, c, funded, gapUsed) // a tapped permanent's Taps trigger (Spider-Gwen's loot) is stacked and asked first
	} else {
		ok = fs.visit(k, c, funded, gapUsed)
	}
	if ok {
		if c.g.stalled != t.g.stalled {
			fs.spill = c.g.stalled
		}
		return true
	}
	if life {
		fs.lifeUsed = fs.lifeUsed.without(st.sl)
	}
	fs.stallOf(c) // the line failed; the fork may have latched a stall (c is live until the deferred unfork)
	fs.n--
	return false
}

// stallOf records the stall a failed line's fork latched (capFull) while the
// line ran, the first one, so filtScript declares it on the real state instead
// of dropping the play as unpayable: the fork cannot go on (its next stepGen
// ends the game), but the play could have been payable along it.
func (fs *filtSearch) stallOf(c *State) {
	if fs.stall == 0 {
		fs.stall = c.g.stalled
	}
}

// stateKey is the visited-state key of node t.
func (fs *filtSearch) stateKey(k *prep, t *State) filtKey {
	return t.scriptKey(k, fs.p, fs.other)
}

// seenTable is the visited set the active search fills: the play search's
// inline table, or the pay search's wider one.
func (fs *filtSearch) seenTable() []filtKey {
	if fs.pay != nil {
		return fs.paySeen
	}
	return fs.seen[:]
}

// seenKey reports whether node t's key is already in the visited set.
func (fs *filtSearch) seenKey(k *prep, t *State) bool {
	key := fs.stateKey(k, t)
	tbl := fs.seenTable()
	for i := 0; i < fs.nseen; i++ {
		if tbl[i] == key {
			return true
		}
	}
	return false
}

// scriptKey is gorge's scriptStateKey of p at node t: the pool by colour, the
// tapped permanents, the counters of the counter-cost gaps other, and the
// stack, hand and graveyard sizes. The Combo Any search keys its nodes by it
// too (caSearch.visit).
func (t *State) scriptKey(k *prep, p uint8, _ slotMask) filtKey {
	key := filtKey{pool: t.pl[SeatIx(p)].pool, sp: t.sp, hand: t.pl[SeatIx(p)].handN, gy: t.pl[SeatIx(p)].graveN}
	// gorge's key (scriptStateKey) reads the pool, a restricted unit included,
	// by colour alone: Pool holds every floating unit, and the restriction
	// that tags it is no part of the key. A restricted {U} beside a Relic's {W}
	// is the state a Relic's {U} beside a restricted {W} is, so the second is
	// a dedup hit there and must be one here, or the search spends nodes
	// gorge's budget does not (CR 601.2g-h).
	var rbPlain [kop.NumManaKinds]uint8
	t.rbUnits(p, t.rbAll(p), &rbPlain)
	for c, n := range rbPlain {
		key.pool[c] += n
	}
	if k.artMana {
		t.artSync(p)
		a := t.g.poolArt[SeatIx(p)]
		key.art = a &^ (poolArtRestr | 1<<kop.NumManaKinds - 1) // the snow tag; the colour slots are typed below
		// the typed units by colour, a restricted batch's included (gorge's
		// ManaUnits: Heirloom's {U} and a Relic's {W} are both artifact mana,
		// whichever made which)
		for c := 0; c < kop.NumManaKinds; c++ {
			if a&(1<<c) != 0 {
				key.typed[c] = key.pool[c] - rbPlain[c]
				// gorge keys the tagged units by count: a Signpost's {G} beside
				// an Elvish Spirit Guide's is not two tagged {G} (coldArtN)
				if sx := coldArtN.of(t); sx != nil && !DebugArtKeyBit {
					if n := sx[int(SeatIx(p))*kop.NumManaKinds+c]; n != 0 && n < key.typed[c] {
						key.typed[c] = n
					}
				}
			}
		}
		if a&poolArtRestr != 0 {
			for c := range key.typed {
				key.typed[c] += rbPlain[c]
			}
		}
	}
	key.bf = t.ctrlM[SeatIx(p)].and(t.occ)
	for b := key.bf; b.any(); b = b.rest() {
		if sl := b.first(); t.perms[sl].flags&pfTapped != 0 {
			key.tapped = key.tapped.with(sl)
		}
	}
	t.ctrKey(k, key.bf, &key)
	return key
}

// skipSlot: sl is not this pass's to activate, or it is already a step of
// this path: the play's own {T} source (CR 602.2b), a dead gap, a life-only
// source outside its last-resort pass, or a life source already paid.
func (fs *filtSearch) skipSlot(k *prep, t *State, sl Slot, gap, freeOnly bool, pass int) bool {
	if (fs.gaps.has(sl) || fs.other.has(sl)) != gap || sl == fs.own || fs.dead.has(sl) {
		return true
	}
	if !gap {
		if fs.a.Kind() == AkCast && fs.a.Mode()&castFace != 0 {
			// A face cast's funding source must be able to pay the face's
			// cost: gorge's prefix search drops a restricted source the
			// payment cannot spend (scriptSpendableGaps, CR 106.6). A plain
			// cast keeps such a source, whose mana may still pay a filter's
			// fee (Hydraulic Helper's artifact-only {U} funding Baxter
			// Building's {4} for Shuri).
			d := t.grantPayDesc(k, fs.a)
			if !t.gapSpendable(k, sl, &d) {
				return true
			}
		}
		return false
	}
	if fs.feeOrder && fs.feeGaps.has(sl) != (pass == 1) {
		return true // the fee gaps are the order's second pass, the others its third
	}
	if freeOnly && !t.canTapForMana(k, sl) {
		return true // a fee-carrying gap is the phased run's third pass
	}
	if fs.lifeUsed.has(sl) {
		return true
	}
	_, life := t.lifeGapSource(k, sl)
	return life
}

// twinTried reports whether an earlier slot this pass is indistinguishable
// from sl (twinSlot): its activation subtree is isomorphic -- the same mana
// for the same cost -- so the branch through sl reaches no payer state the
// earlier one's does not (gorge's scriptTwin, potential_script.go, cut E).
func (fs *filtSearch) twinTried(k *prep, t *State, sl Slot, tried []Slot) bool {
	for _, a := range tried {
		if fs.twinSlot(k, t, a, sl) {
			return true
		}
	}
	return false
}

// twinSlot reports whether two battlefield slots are indistinguishable mana
// sources for the search: the same card, both untapped, no counters and
// nothing attached to either. gorge's scriptTwin requires the same printed
// name and the same activate options; the card definition plus the shared
// abilities is that same pair (a granted ability is read through the same
// card's state, so a grant that reaches one reaches the other).
func (fs *filtSearch) twinSlot(k *prep, t *State, a, b Slot) bool { return twinSlotOf(t, a, b) }

// twinSlotOf is twinSlot's test, which reads no search state: the grant
// search's funding loop asks it too (grantSrch.fundTwinTried).
func twinSlotOf(t *State, a, b Slot) bool {
	if a == b || !t.occ.has(a) || !t.occ.has(b) {
		return false
	}
	pa, pb := &t.perms[a], &t.perms[b]
	if pa.flags&pfTapped != 0 || pb.flags&pfTapped != 0 {
		return false
	}
	if t.cardAt(a) != t.cardAt(b) {
		return false
	}
	if pa.p1 != 0 || pb.p1 != 0 || pa.cn != ([ctrSlots]uint16{}) || pb.cn != ([ctrSlots]uint16{}) ||
		pa.ck != ([ctrSlots]uint16{}) || pb.ck != ([ctrSlots]uint16{}) {
		return false
	}
	if pa.attached != NoSlot || pb.attached != NoSlot {
		return false
	}
	// gorge's scriptTwin also compares the activate options the decision
	// offers: an ability one source lost or was granted (an override record,
	// a layer-6 effect) and the other did not makes them two different
	// subtrees. Fail open on any such difference.
	if pa.ovr != pb.ovr || t.fxM.has(a) != t.fxM.has(b) {
		return false
	}
	for s2 := Slot(0); s2 < MaxPerm; s2++ {
		if t.occ.has(s2) && (t.perms[s2].attached == a || t.perms[s2].attached == b) {
			return false
		}
	}
	return true
}

// deadFunder: sl is a dropped any-colour restricted gap (Gwenna beside a
// planeswalker cast: its creature-only mana is spendable by no part of the
// play) still untapped and activatable at this node. gorge's one prefix
// search keeps it among the plain activations (scriptSpendableGaps removes it
// from the census gaps only; the funding pass walks every option whose
// source yields mana), so it is a funding step of the search that floats
// mana the play refuses, charged to the node budget but never paying a fee.
func (fs *filtSearch) deadFunder(k *prep, t *State, sl Slot) bool {
	return fs.dead.has(sl) && sl != fs.own && t.scriptAnyR(k, fs.p).has(sl)
}

// deadFund branches dropped gap sl's answers as a funding activation: the
// same asks the gap's own branch poses (anyRGap), one funded step each.
func (fs *filtSearch) deadFund(k *prep, t *State, sl Slot, funded int, gapUsed bool) bool {
	return fs.anyRGap(k, t, sl, func(o otherStep) bool {
		return fs.try(k, t, filtStep{sl: sl, other: o}, funded+1, gapUsed)
	})
}

// lifePass activates one life-only gap of the node's order: the last resort,
// after the gaps and the funding sources (gorge pays a filter's fee from a
// land before it pays life).
func (fs *filtSearch) lifePass(k *prep, t *State, order []Slot, funded int) bool {
	for _, sl := range order {
		if !fs.other.has(sl) || sl == fs.own || fs.dead.has(sl) || fs.lifeUsed.has(sl) {
			continue
		}
		if _, ok := t.lifeGapSource(k, sl); !ok {
			continue
		}
		if fs.otherGap(k, t, sl, funded) {
			return true
		}
	}
	return false
}

// restrictedAnswers tries a restricted ability's colour answers (rm) of the
// source at sl, after the census one's, the play's announced colours first
// (gorge's scriptOrderAnswers on the same colour ask, CR 601.2b).
func (fs *filtSearch) restrictedAnswers(k *prep, t *State, sl Slot, rm uint8, funded int, gapUsed bool) bool {
	var order [kop.NumManaKinds]uint8
	no := 0
	for m := rm; m != 0; m &= m - 1 {
		order[no] = uint8(bits.TrailingZeros8(m))
		no++
	}
	needFirst(order[:no], fs.need)
	for _, c := range order[:no] {
		if fs.try(k, t, filtStep{sl: sl, ans: c, restr: true}, funded+1, gapUsed) {
			return true
		}
	}
	return false
}

// offers reports whether the play is a legal option at t's priority node
// (visit's scan, before the visited set).
func (fs *filtSearch) offers(k *prep, t *State) bool {
	var d Decision
	t.restrOptions(k, &d, fs.p) // (restricted mana floating is priced as ordinary: the search reads no restriction)
	t.capGrantOvercounts(k, &d, fs.p)
	return hasOption(&d, fs.a) && !t.grantRefusedPrices(k, fs.p, fs.a) && !fs.witnessEats(k, t)
}

// visit is one priority node of the search.
// terminal reports whether the search reached its goal at t: a payment's
// pool pays fs.pay, or a play's offer lists fs.a. The play's comment is the
// original offer check: an activation's prefix is its payment window, so a
// filter gap whose own tap queues a Taps trigger keeps tapping until the
// floating pool pays the ability's reduced cost (gorge's manaWindowAsk runs
// before the push).
func (fs *filtSearch) terminal(k *prep, t *State, gapUsed bool) bool {
	if fs.pay != nil {
		return t.poolPays(fs.p, fs.pay)
	}
	if !(gapUsed || fs.hands > 0) || !fs.offers(k, t) { // (a guide's mana makes the raw priority list the play with no gap used)
		return false
	}
	// A filter gap whose own tap queues a Taps trigger is the window's
	// colour-asked tap: the offer still counting it untapped has not
	// activated it yet, so keep tapping until the floating pool pays the
	// ability's reduced cost. Its trigger then lands below the ability, as
	// gorge's does. A filter whose tap queues no trigger, and every cast,
	// keeps the offer check (the planner pays those after the push by the
	// same convention).
	return fs.a.Kind() != AkActivate || !fs.deferGapLeft(k, t) || t.poolPaysPlay(k, fs.p, fs.a)
}

// orderFunders puts a payment search's fixed-colour funders before its
// flexible ones: a fixed source fills a pip the flexible one would otherwise
// take (Overflowing Basin's {1} from Power Depot's {C}, then Lantern of
// Revealing's any colour for the last pip). Gorge's planner ranks the fixed
// sources first too.
func (fs *filtSearch) orderFunders(k *prep, t *State, order []Slot, gap bool) {
	if gap || fs.pay == nil {
		return
	}
	var fixed, flex [MaxPerm]Slot
	nf, nx := 0, 0
	for i := range order {
		sl := order[i]
		if bits.OnesCount8(t.fundMaskAt(k, t.manaPCGen(k, sl), sl)) > 1 {
			flex[nx] = sl
			nx++
		} else {
			fixed[nf] = sl
			nf++
		}
	}
	copy(order[:nf], fixed[:nf])
	copy(order[nf:], flex[:nx])
}

// passCount is the passes of visit's activation loop: the plain run's gaps,
// funding sources and life-only source, and the phased run's (gorge's
// fallback) gaps whose ability costs no mana, funding sources, gaps, then the
// life-only source.
func (fs *filtSearch) passCount() int {
	if fs.phased {
		return 4
	}
	return 3
}

// passShape reports whether visit's pass walks the gaps, and whether it is
// skipped: the funding passes are once the funding is spent (gorge's phase
// loop breaks there, so the phased run's last gap pass goes with it).
func (fs *filtSearch) passShape(pass, funded int, feeGapsOut *bool) (gap, freeOnly, skip bool) {
	if fs.feeOrder {
		return pass != 0, false, pass == 0 && funded >= fs.fees // the funding passes go once it is spent; the fee gaps that follow pay from it
	}
	gap = pass == 0 || fs.phased && pass == 2
	freeOnly = fs.phased && pass == 0
	if gap {
		return gap, freeOnly, *feeGapsOut
	}
	fees := fs.fees
	if fs.fundCap > 0 && fs.fundCap < fees {
		fees = fs.fundCap
	}
	if funded >= fees {
		*feeGapsOut = fs.phased
		return gap, freeOnly, true
	}
	return gap, freeOnly, false
}

// holdDualFilter reports whether sl is a lone dual filter on a cast whose
// fee the funding pass can pay: its free {C} tap must wait, or it spends the
// 160-node budget before the fee floats and the paid activation is never
// reached (Baxter Building beside four lands, King T'Challa). A board with a
// second filter or with too few funders keeps the plain walk's free tap.
func (fs *filtSearch) holdDualFilter(k *prep, t *State, sl Slot) bool {
	return fs.gaps.count() == 1 && fs.a.Kind() == AkCast && fs.fees > 0 &&
		fs.fundN >= fs.fees && comboFiltAb(t.filtAb(k, sl))
}

func (fs *filtSearch) visit(k *prep, t *State, funded int, gapUsed bool) bool {
	if !fs.tick() {
		return false
	}
	p := fs.p
	// Price the node before the visited set, as gorge's scriptSearch.visit
	// offers before it dedups: the colour-only key cannot tell a restricted
	// unit from a filter's converted one, so a priced node can key equal to an
	// earlier unpriced sibling.
	if fs.terminal(k, t, gapUsed) {
		return true
	}
	key := fs.stateKey(k, t)
	tbl := fs.seenTable()
	for i := 0; i < fs.nseen; i++ {
		if tbl[i] == key {
			return false
		}
	}
	if fs.nseen < len(tbl) {
		tbl[fs.nseen] = key
		fs.nseen++
	}
	var order [MaxPerm]Slot
	n := t.battlefieldOrder(p, t.ctrlM[SeatIx(p)].and(t.occ), order[:], 0)
	passes := fs.passCount()
	feeGapsOut := false
	for pass := 0; pass < passes; pass++ {
		// pass 0: the gaps; pass 1: the funding sources; pass 2: a life-only
		// source, the last resort (gorge pays a filter's fee from a land
		// before it pays life, so a life gap must not pre-empt that funding).
		if pass == passes-1 {
			if fs.lifePass(k, t, order[:n], funded) {
				return true
			}
			continue
		}
		gap, freeOnly, skip := fs.passShape(pass, funded, &feeGapsOut)
		if skip {
			continue
		}
		fs.orderFunders(k, t, order[:n], gap)
		var tried [MaxPerm]Slot
		ntried := 0
		for i := 0; i < n; i++ {
			sl := order[i]
			if !gap && fs.deadFunder(k, t, sl) {
				if fs.deadFund(k, t, sl, funded, gapUsed) {
					return true
				}
				continue
			}
			if fs.skipSlot(k, t, sl, gap, freeOnly, pass) {
				continue
			}
			if gap && fs.other.has(sl) {
				if fs.otherGapTwin(k, t, sl, funded, &tried, &ntried) {
					return true
				}
				continue
			}
			if (fs.feeOnly || isAbilityPlay(fs.a)) && fs.twinTried(k, t, sl, tried[:ntried]) {
				continue // an indistinguishable earlier source already walked this subtree
			}
			if gap && k.gen.anyFiltFirst {
				if !t.filtUsable(k, sl) {
					continue // (a once-each-turn filter: its use, not its {T})
				}
			} else if t.fundOut(k, sl) {
				continue
			}
			// Recorded once the slot is usable, as gorge's scriptTwin list is
			// (after its availability gates): an earlier source that could not
			// be activated walked no subtree its later twin would repeat.
			tried[ntried] = sl
			ntried++
			if gap {
				free := t.canTapForMana(k, sl)
				filt := t.filtPaid(k, p, sl)

				if handled, res := fs.gapFreeSpecial(k, t, sl, funded, free, filt); handled {
					if res {
						return true
					}
					continue
				}
				switch {
				case free && filt:
					// two abilities: the ask, its {C} answer first
					if !fs.tick() {
						return false
					}
					if fs.try(k, t, filtStep{sl: sl, gap: true}, funded, true) {
						return true
					}
					if fs.filtColours(k, t, sl, funded) {
						return true
					}
				case free:
					if fs.holdDualFilter(k, t, sl) {
						continue
					}
					if fs.try(k, t, filtStep{sl: sl, gap: true}, funded, true) {
						return true
					}
				case filt:
					if fs.feeOnly && t.filtRepeat(k, sl) && fs.activationStalls(k, t, sl) {
						continue // every colour the fee-only repeatable activation can make is doomed
					}
					if fs.filtColours(k, t, sl, funded) {
						return true
					}
				}
				continue
			}
			rm := uint8(0)
			if k.restrSrc {
				rm = t.filtRestrMask(k, sl)
				if rm != 0 && fs.census && fs.pay != nil && !t.filtRestrOtherAdmits(k, p, sl) {
					rm = 0 // the unless / ward census lists no restricted source its restriction bars from a non-spell payment
				}
			}
			free := t.fundFree(k, sl)
			if !free && rm == 0 {
				if t.canSacOnlyNowAt(k, sl) && fs.try(k, t, filtStep{sl: sl, sacOnly: true}, funded+1, gapUsed) {
					return true // a Spawn funds beside the Treasures (mana_filtfund.go)
				}
				continue
			}
			if res, stop := fs.fundTap(k, t, sl, rm, free, funded, gapUsed); stop {
				return res
			}
		}
	}
	// The unless / ward census prices no hand mana (unlessWindowUnits), so a
	// census pay search never reaches a hand guide: gorge offers decline where
	// only the guide could pay. A plain window's pay probe keeps the guide.
	if (!fs.census || fs.pay == nil) && funded < fs.fees && handManaOn(k) && fs.handFund(k, t, funded, gapUsed) {
		return true // a hand guide funds after the battlefield's sources (mana_handguide.go)
	}
	return fs.passTrigger(k, t, funded)
}

// fundTap: one funding tap of sl in visit's pass: its asks and answers in
// gorge's order. stop reports that visit ends with res (a plan found, or the
// node budget spent); otherwise the pass goes on to the next slot.
func (fs *filtSearch) fundTap(k *prep, t *State, sl Slot, rm uint8, free bool, funded int, gapUsed bool) (res, stop bool) {
	if free && rm != 0 && !fs.tick() {
		return false, true // the ask between its census and its restricted ability
	}
	zeroAsk := free && rm == 0 && fs.zeroFiltAsk(k, t, sl)
	if free {
		pc := t.manaPCGen(k, sl)
		mask := t.fundMaskAt(k, pc, sl) // a painland asks its painful colours too (mana_filtpain.go)
		if bits.OnesCount8(mask) == 1 && zeroAsk && fs.zeroFiltAsks(k, t, sl) {
			// the tap is the new gate's own (fundOne): its ask is charged
			// there once, and the old gate's would charge the same tap's ask
			// a second time
			zeroAsk = false
		}
		if zeroAsk && !fs.tick() {
			return false, true // the ask between its free ability and a zero filter (mana_zerofilt.go)
		}
		if bits.OnesCount8(mask) > 1 && !fs.tick() {
			return false, true // the colour ask
		}
		if bits.OnesCount8(mask) > 1 {
			// a Treasure funds too, sacrificed; the ask in gorge's order (mana_filtfund.go)
			if fs.fundColours(k, t, sl, pc, mask, funded, gapUsed) {
				return true, true
			}
		} else if fs.fundOne(k, t, sl, pc, mask, funded, gapUsed) {
			return true, true
		}
	}
	if bits.OnesCount8(rm) > 1 && !fs.tick() {
		return false, true // the restricted ability's colour ask
	}
	if fs.restrictedAnswers(k, t, sl, rm, funded, gapUsed) {
		return true, true
	}
	return false, false
}

// passTrigger: a trigger a step caused: let it resolve, the pool floats (gorge
// scriptSearch.visit, the last resort at a node; a sorcery-speed play waits
// for it, waitsForStack).
func (fs *filtSearch) passTrigger(k *prep, t *State, funded int) bool {
	if t.sp <= fs.sp0 || fs.n >= maxFiltPath {
		return false
	}
	pass := MakeAction(AkPass, 0, 0, 0)
	c := t.fork()
	c.Apply(pass)
	fs.path[fs.n] = filtStep{sl: NoSlot, other: otherStep{kind: otherPass}, act: pass}
	fs.n++
	ok := fs.settle(k, c, funded, true)
	if !ok {
		fs.stallOf(c)
	}
	t.unfork()
	if ok {
		return true
	}
	fs.n--
	return false
}

// witnessEats: the node prices the cast play only by a plan that sacrifices
// the play's only target (Portent of Betrayal's Catalyst Elemental, the one
// creature, paying its {R}{R}). gorge's scriptSearch.priced replays a plan that
// consumes a permanent (witnessReachesFrom) and takes the node only when the
// play is still offered after it, so the search goes on (CR 601.2c: a target
// must remain legal). xWitnessEats is that replay for a spell.
func (fs *filtSearch) witnessEats(k *prep, t *State) bool {
	if !isCastKind(fs.a.Kind()) || fs.a.Mode() != 0 || !t.anySacSource(k, fs.p) {
		return false
	}
	o := Obj(fs.a.Source())
	if o == NoObj || int(o) >= len(t.objs) {
		return false
	}
	cost, ok := t.plannedCastCost(k, fs.p, o, 0)
	return ok && t.xWitnessEats(k, fs.p, t.objs[o].card, &cost, 0, o, 0)
}

// fundOne taps the one-colour funding source sl for its census ability, then
// for a gate-on pair's larger one: gorge's ability ask lists both and the
// search takes them in turn (filtStep.wide).
func (fs *filtSearch) fundOne(k *prep, t *State, sl Slot, pc *pcard, mask uint8, funded int, gapUsed bool) bool {
	zero := fs.zeroFiltAsks(k, t, sl)
	if zero && !fs.tick() {
		return false // the ask beside a zero filter (mana_filtzerofund.go)
	}
	ans := uint8(bits.TrailingZeros8(mask))
	if fs.try(k, t, filtStep{sl: sl, ans: ans}, funded+1, gapUsed) {
		return true
	}
	// the pair's wide answer after the zero filter's ask: gorge's ability ask
	// lists both, and the search takes them in turn (filtStep.wide). No current
	// card pairs a zero dynamic filter with a gate-on pair, so the order is
	// untested against gorge's; the wide is kept rather than dropped.
	return t.widePairAb(k, sl, pc) >= 0 && fs.tick() && fs.try(k, t, filtStep{sl: sl, ans: ans, wide: true}, funded+1, gapUsed)
}

// DebugFiltNoTrigSettle leaves a Taps trigger a filter-search step caused
// unstacked (mutation testing only: the lockstep must catch it).
var DebugFiltNoTrigSettle bool

// settle continues the search at t after a step that queued a trigger, or a
// pass: t runs to p's next decision, a priority being a node (visit) and an
// ask a node branching on its answers (gorge scriptSearch.visit's
// scriptAnswers; tapXSearch.settle).
func (fs *filtSearch) settle(k *prep, t *State, funded int, gapUsed bool) bool {
	var d Decision
	if !t.tapXNext(fs.p, &d) {
		fs.stallOf(t)
		return false
	}
	if d.Kind == DkPriority {
		if fs.visit(k, t, funded, gapUsed) {
			return true
		}
		fs.stallOf(t)
		return false
	}
	if !fs.tick() { // the ask
		return false
	}
	if d.Kind == DkTarget {
		t.gorgeTargetOrder(&d)
	}
	if !gorgeAskOrder(&d) {
		return false
	}
	for i := 0; i < int(d.N) && fs.n < maxFiltPath; i++ {
		c := t.fork()
		c.Apply(d.Options[i])
		fs.path[fs.n] = filtStep{sl: NoSlot, other: otherStep{kind: otherAnswer}, act: d.Options[i]}
		fs.n++
		ok := fs.settle(k, c, funded, gapUsed)
		if !ok {
			fs.stallOf(c)
		}
		t.unfork()
		if ok {
			return true
		}
		fs.n--
	}
	return false
}

// filtColours is the filter's colour ask at sl: the colours in turn, Any's
// W U B R G or a Combo's in printed order (manaColourOrder; gorge's
// askManaColor offers effects.ComboColours in script order). A combination
// filter (Baxter Building) asks for a whole allocation instead (filtCombo).
// gapOneShot reports whether sl's colour ask is a one-shot flexible gap's
// (a gap source that is not the fee-only repeatable class filtFeeColStalls
// reads): gorge's oneShotAskPrune class (potential_script.go). The prune drops
// a colour that fills no fixed pip while one is owed, reading the
// activation's opening pool, so an ability fee (Henge of Ramos's {2},{T}) is
// in the class too; the fee-only class fails open.
func (fs *filtSearch) gapOneShot(k *prep, t *State, sl Slot) bool {
	return fs.usefulOK && fs.gaps.has(sl) && !t.filtRepeat(k, sl)
}

// gapFreeSpecial handles the two gap-slot shapes visit settles before its
// free/filter switch. handled reports the slot was settled here (visit then
// returns true when res is set, else moves to the next slot): a restricted
// ability beside the filter (mana_mixsrc.go), or a free ability whose
// activation spends the play's only producer of a needed pip
// (freeBranchHopeless), where the paid ability is taken now when its fee is
// paid, else the source is left to a later funding tap.
func (fs *filtSearch) gapFreeSpecial(k *prep, t *State, sl Slot, funded int, free, filt bool) (handled, res bool) {
	if rm := t.mixSrcMask(k, sl); free && rm != 0 {
		return true, fs.mixSrcGap(k, t, sl, funded, rm, filt)
	}
	if free && fs.freeBranchHopeless(k, t, sl) {
		return true, filt && fs.filtColours(k, t, sl, funded)
	}
	return false, false
}

// freeBranchHopeless reports whether activating sl's free mana ability cannot
// help the searched play: a coloured pip the play's printed cost names can be
// produced by neither that free ability, the pool, nor any other untapped
// source. The free branch would then spend the play's only producer of that
// colour for mana the play cannot use, and every continuation below it is
// dead. Skipping it keeps the bounded search out of that subtree so a paid
// ability of the same source can be funded (Vault of the Archangel beside
// Unknown Shores' {1},{T}: Add one mana of any colour: the free {C} answer
// spent the only white source and the ~150-node subtree below it exhausted
// potentialScriptNodes before the paid answer's funding line, which gorge's
// plain run finds in 70 nodes; tail-34 pair 25, seed 255781).
//
// The scan over-approximates every route the branch's continuations could
// use: the pool, every other untapped permanent's printed mana abilities
// (their costs ignored), intrinsic granted land types, and chosen colours.
// Any route it does not model -- a hand mana guide, a live grant, another gap
// class (an each-colour or chosen pair, a tap-another, counter, life or
// distinct-colour source), a StatTapExtra carrier whose extra rides the tap
// itself (mana_tapextra.go: the scan reads printed production, not the extra
// the tap brings, and the free tap's own extra is exactly a route it misses)
// -- fails open, as does a cost whose printed pips the search does not read
// (an X, hybrid, snow or Phyrexian cost): the prune must never drop a branch
// a real plan could use.
func (fs *filtSearch) freeBranchHopeless(k *prep, t *State, sl Slot) bool {
	if !fs.usefulOK || k.gen == nil || !k.gen.anyFilt || k.gen.anyTapExtra || k.gen.anyAnyAct || k.gen.anyContGrant ||
		len(k.gen.grantPC) != 0 || handManaOn(k) || t.grantsLive(k) {
		return false
	}
	if t.scriptAnyR(k, fs.p).or(t.scriptTapX(k, fs.p)).or(t.scriptCtr(k, fs.p)).or(t.lifeGapMask(k, fs.p)).
		or(t.anyDifferentGaps(k, fs.p)).or(t.scriptGaps(k, fs.p)).or(t.chosenPairGaps(k, fs.p)).any() {
		return false // another gap class beside the filters: its route is not read here
	}
	need := false
	for _, b := range fs.need {
		need = need || b
	}
	if !need {
		return false
	}
	p := fs.p
	var reach uint8
	pl := &t.pl[SeatIx(p)]
	for c := 0; c < kop.NumManaKinds; c++ {
		if pl.pool[c] > 0 {
			reach |= 1 << c
		}
	}
	reach |= t.srcMaskAt(t.manaPCGen(k, sl), sl) // the free ability this branch activates
	for b := t.ctrlM[SeatIx(p)].and(t.occ); b.any(); b = b.rest() {
		o := b.first()
		if o == sl || o == fs.own || t.perms[o].flags&pfTapped != 0 {
			continue
		}
		if t.canTapForMana(k, o) {
			reach |= t.srcMaskAt(t.manaPCGen(k, o), o) // a granted basic land type's intrinsic unit too
		}
		cd := &k.t.Cards[t.cardAt(o)]
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 {
				reach |= uint8(a.ManaColors)
			}
		}
		if ch := t.perms[o].chosen; ch < 5 {
			reach |= 1 << ch
		}
	}
	for c := 0; c < kop.NumManaKinds; c++ {
		if fs.need[c] && reach&(1<<c) == 0 {
			return true
		}
	}
	return false
}

func (fs *filtSearch) filtColours(k *prep, t *State, sl Slot, funded int) bool {
	if t.filtAb(k, sl).ManaEach {
		// a fixed production: no colour ask, one activation (mana_filtereach.go)
		return fs.try(k, t, filtStep{sl: sl, ans: 1, gap: true}, funded, true)
	}
	if !t.filtFixed(k, sl) && !fs.tick() { // (a fixed production asks no colour)
		return false
	}
	if t.filtFixed(k, sl) {
		// one fixed colour, {C} included (manaColourOrder names WUBRG only)
		c := uint8(bits.TrailingZeros8(uint8(t.filtAb(k, sl).ManaColors)))
		return fs.try(k, t, filtStep{sl: sl, ans: 1 + c, gap: true}, funded, true)
	}
	if comboFiltAb(t.filtAb(k, sl)) {
		return fs.filtCombo(k, t, sl, funded)
	}
	var order [5]uint8
	n := manaColourOrder(t.filtAb(k, sl), &order)
	needFirst(order[:n], fs.need)
	oneShot := fs.feeOnly && fs.gapOneShot(k, t, sl)
	if oneShot {
		// gorge's oneShotAskPrune fails open when no answer fills a pip
		// (the exchange would otherwise empty the ask): drop the prune
		// unless at least one colour the ask offers is useful.
		any := false
		for i := 0; i < n; i++ {
			if !t.filtOneShotColStalls(k, fs.p, sl, &fs.useful, int(order[i])) {
				any = true
				break
			}
		}
		oneShot = any
	}
	for i := 0; i < n; i++ {
		c := order[i]
		if n > 1 && oneShot && t.filtOneShotColStalls(k, fs.p, sl, &fs.useful, int(c)) {
			continue // a one-shot gap's colour that fills no fixed pip
		}
		if n > 1 && fs.usefulOK && !fs.phased && t.filtFeeColStalls(k, fs.p, sl, &fs.useful, int(c), fs.own) {
			// the fee-only repeatable activation making c is doomed on this
			// node (filtFeeColStalls: c fills no fixed pip the play's cost
			// still lacks once the fee is paid): the ask offers the other
			// colours (gorge's scriptSearch.feeOnlyAskPrune). A fixed or
			// ManaEach production asks no colour, so the branches above
			// never prune, as gorge's ask never happens there.
			continue
		}
		if fs.try(k, t, filtStep{sl: sl, ans: uint8(1 + c), gap: true}, funded, true) {
			return true
		}
	}
	return false
}

// activationStalls reports whether every colour the fee-only repeatable
// filter at sl can make is doomed at the node (filtFeeColStalls): the
// activation and its ask are then never visited (gorge's
// scriptSearch.feeOnlyGapDoomed, potential_script.go, cut A). A shape the
// clamp cannot read -- an unpayable fee, an any-type rider, a producible
// colour ManaColors does not name -- fails open.
func (fs *filtSearch) activationStalls(k *prep, t *State, sl Slot) bool {
	if !fs.usefulOK || fs.phased { // (the fee-stall prune is the plain run's alone)
		return false
	}
	ab := t.filtAb(k, sl)
	any := false
	for c := 0; c < kop.NumManaKinds; c++ {
		if uint8(ab.ManaColors)&(1<<c) == 0 {
			continue
		}
		any = true
		if !t.filtFeeColDoomed(k, fs.p, sl, &fs.useful, c, fs.own) {
			return false
		}
	}
	return any
}

// filtCombo is the allocation ask of a combination filter at sl: gorge's
// scriptAnswers enumerates a Min == Max == n colour allocation unit by unit,
// the last unit varying fastest, and stops at 64 combinations, those naming a
// needed colour first (scriptOrderAnswers). Each is one
// activation producing its counts (filtStep.alloc); the first whose
// production pays later is the script's.
func (fs *filtSearch) filtCombo(k *prep, t *State, sl Slot, funded int) bool {
	n := int(t.filtAb(k, sl).ManaN)
	if n <= 0 || n > 6 {
		return false
	}
	const per = 5 // Any's W U B R G, the order askManaColor offers
	// gorge's scriptAnswers lists the first 64 per-unit combinations, last
	// unit fastest; scriptOrderAnswers then tries first the ones where some
	// unit names a colour the play's cost needs (the order decides which
	// plans the node budget reaches), engine order kept inside each half.
	var combos [64][6]uint8
	nc := 0
	var idx [6]uint8
	for nc < len(combos) {
		combos[nc] = idx
		nc++
		u := n - 1
		for ; u >= 0; u-- {
			if idx[u]++; idx[u] < per {
				break
			}
			idx[u] = 0
		}
		if u < 0 {
			break
		}
	}
	wanted := func(c *[6]uint8) bool {
		for u := 0; u < n; u++ {
			if fs.need[c[u]] {
				return true
			}
		}
		return false
	}
	for pass := 0; pass < 2; pass++ {
		for i := 0; i < nc; i++ {
			if wanted(&combos[i]) != (pass == 0) {
				continue
			}
			var st filtStep
			st.sl, st.ans, st.gap = sl, 1, true
			for u := 0; u < n; u++ {
				st.alloc[combos[i][u]]++
			}
			if fs.try(k, t, st, funded, true) {
				return true
			}
		}
	}
	return false
}

// gapFilteredPlay: play a is one whose prefix search keeps only the gaps
// its payment can spend (gorge scriptSpendableGaps): a plain cast of the
// printed cost (from the hand, or under a may-play permission that only
// opens the zone: Summoning Materia's top of the library), or an activation.
func gapFilteredPlay(a Action) bool {
	return a.Kind() == AkActivate || a.Kind() == AkCast && (a.Mode() == 0 || a.Mode() == castMayPlay)
}

// isAbilityPlay: action a is one of the activated plays gorge's twin cut
// runs for -- a printed activation or a gained one (the 0xC0 mode pair,
// static_gainsof.go), the two option shapes gorge's potentialTwinCut
// classifies as printed or gained. The 0x80 bit alone is every grant dirac
// cannot tell apart by encoding: a max-speed grant (legal_gen.go) and an
// exile grant (cast_exilegrant.go) are Kind "granted", but a non-max-speed
// SVar grant (Fishing Pole's ability granted to the equipped creature) is
// Kind "ability" too. Gorge narrows its cut to the printed/gained option
// shapes precisely because dirac cannot split those grants, so the cut must
// not run on 0x80 here.
func isAbilityPlay(a Action) bool {
	if a.Kind() != AkActivate {
		return false
	}
	return a.Mode()&0x80 == 0 || isGainedMode(a.Mode())
}

// gapSpendableMask drops from m each source whose every mana ability, printed
// or granted, carries a spend restriction payment d refuses (CR 106.6):
// gorge's prefix search (scriptSpendableGaps) never activates such a gap for
// the play, since it can only float mana the payment may not spend.
func (s *State) gapSpendableMask(k *prep, m slotMask, d *payDesc) slotMask {
	out := m
	for b := m; b.any(); b = b.rest() {
		if sl := b.first(); !s.gapSpendable(k, sl, d) {
			out = out.andNot(slotMask{}.with(sl))
		}
	}
	return out
}

// gapSpendable: some mana ability of the source at sl, printed or granted,
// is unrestricted or admits payment d (gorge gapSpendableFor's walk of the
// census's own membership, which reads every mana ability).
func (s *State) gapSpendable(k *prep, sl Slot, d *payDesc) bool {
	p := s.perms[sl].ctrl
	if s.landSetLost(k, sl) {
		// a land under a land-type setter (Contaminated Ground) lost its own
		// abilities but taps for the intrinsic mana of its new basic type (CR
		// 305.7, 305.6), which carries no spend restriction
		return true
	}
	if !s.genLost(k, sl) {
		cd := &k.t.Cards[s.cardAt(sl)]
		for j := range cd.Abilities {
			if a := &cd.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 &&
				(a.ManaRestrict == 0 || s.restrSrcAdmits(k, p, sl, a, d)) {
				return true
			}
		}
	}
	if k.gen == nil || len(k.gen.grantPC) == 0 {
		return false
	}
	var gi [maxGrants]int16
	for i, n := 0, s.grantIdxs(k, sl, &gi); i < n; i++ {
		r := k.gen.grantOf[gi[i]]
		if k.gen.grantRestr[gi[i]] == 0 || s.restrSrcAdmits(k, p, sl, &k.t.Cards[r.card].Abilities[r.ab], d) {
			return true
		}
	}
	return false
}
