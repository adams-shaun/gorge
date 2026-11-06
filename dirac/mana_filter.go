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
	if a.Kind != kop.AbMana || a.AFlags&^(kop.AfManaDynamic|kop.AfManaChoice) != 0 || a.ManaRestrict != 0 || a.Cost.Flags != kop.CostTap ||
		m.Generic == 0 || m.X != 0 || m.Hyb[0] != 0 || m.Pips != ([kop.NumManaKinds]uint8{}) ||
		a.ManaColors != kop.ColAny || a.ManaN == 0 || a.ManaN > 1 && (a.AFlags&kop.AfManaChoice == 0 || !comboFiltAb(a)) {
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
	return 1
}

// filtRelaxable: gorge's relaxed proof can enumerate the filters f: at most
// relaxedPaidLimit, each dynamic amount at least one and at most six
// (rules/potential_plan.go paymentPlanRelaxedAlternatives).
func (s *State) filtRelaxable(k *prep, f slotMask) bool {
	if f.count() > relaxedPaidLimit {
		return false
	}
	for ; f.any(); f = f.rest() {
		if n := s.filtUnits(k, f.first()); n < 1 || n > 6 {
			return false
		}
	}
	return true
}

// availFilt: in the gaps walk each filter is a free unit of its colours
// (any, or a Combo's) and its fee is charged on top of the cost (paymentPlanRelaxProof's run with
// every paid gap: the most permissive of its subsets, since an any-colour
// unit only helps). Charging the fee is taking it off the total.
//
//go:noinline
func (s *State) availFilt(k *prep, p uint8, a *avail) {
	for b := s.scriptFilt(k, p); b.any(); b = b.rest() {
		a.addUnits(uint8(s.filtAb(k, b.first()).ManaColors), uint8(s.filtUnits(k, b.first())))
		a.total -= s.filtFee(k, b.first())
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
	for set := 1; set < 1<<n; set++ {
		t := *av
		for i := 0; i < n; i++ {
			if set&(1<<i) == 0 {
				continue
			}
			sl := filt[i]
			if !s.filtNoTap(k, sl) && s.canTapForMana(k, sl) {
				pc := s.manaPCGen(k, sl)
				s.removeSrc(k, &t, sl, pc)
			}
			// the fee comes first, out of what is made so far (CR 605.3b)
			fee := s.filtFee(k, sl)
			if t.total < fee {
				t.total = -1 << 30
				break
			}
			t.total -= fee
			t.addUnits(uint8(s.filtAb(k, sl).ManaColors), uint8(s.filtUnits(k, sl)))
		}
		if t.affords(r, pipMaskOf(r), extra) {
			return true
		}
	}
	return false
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
	best := -1
	for set := 1; set < 1<<n; set++ {
		t := *av
		if !s.filtActivate(k, &t, &filt, set) {
			continue
		}
		if anyColor {
			best = max(best, t.maxXAnyColor(cost))
		} else {
			best = max(best, t.maxX(cost, pipMaskOf(cost)))
		}
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

// filtActivate activates the filters of set (bits of filt) on t: each gives up
// its free ability's mana, pays its fee out of the mana made so far and adds
// its own. The fees are paid in an order that funds them, the filters that
// add at least what they cost first; false when some fee cannot be paid.
func (s *State) filtActivate(k *prep, t *avail, filt *[maxFaceFilters]Slot, set int) bool {
	for pass := 0; pass < 2; pass++ {
		for progress := true; progress && set != 0; {
			progress = false
			for i := range filt {
				if set&(1<<i) == 0 {
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
				if pass == 0 && units < ownN+fee || t.total-ownN < fee {
					continue
				}
				if pc != nil {
					s.removeSrc(k, t, sl, pc)
				}
				t.total -= fee
				a := s.filtAb(k, sl)
				t.addUnits(uint8(a.ManaColors), uint8(units))
				if manaOneColour(a, uint8(units)) {
					s.addOneColour(t, uint8(units))
				}
				if !comboFiltAb(a) {
					// the filter's tap brings the TapsForMana extra
					// (mana_tapextra.go filtExtra); a combination filter's
					// activation keeps the stall (filtDo)
					xv := s.filtExtra(k, sl)
					for c := range xv {
						if xv[c] != 0 {
							t.addUnits(1<<c, xv[c])
						}
					}
				}
				set &^= 1 << i
				progress = true
			}
		}
	}
	return set == 0
}

// windowFiltUseful: sl's filter can make a colour in want now, its fee
// covered by p's floating mana (gorge's paymentSourceUseful reads the window
// abilities, and a filter's is listed only once the pool pays it).
func (s *State) windowFiltUseful(k *prep, p uint8, sl Slot, want uint8) bool {
	return k.gen != nil && k.gen.has(s.cardAt(sl), gfFilt) && uint8(s.filtAb(k, sl).ManaColors)&want != 0 &&
		poolTotal(&s.pl[p&1]) >= s.filtFee(k, sl)
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
func (s *State) windowRestrOwedTap(k *prep, p uint8, sl Slot, useful *kop.ManaCost) bool {
	want, decided := s.windowUseful(k, p, useful)
	if !decided || s.windowSrcMask(k, sl)&want != 0 {
		return false
	}
	cd := &k.t.Cards[s.cardAt(sl)]
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		// An any-colour restricted ability (White Lotus Hideout's
		// Lesson/Shrine mana) is a choice-shaped Produced$ Any: gorge's
		// owedAbilitiesFirst lists it first when the payment owes a colour
		// its plain ability cannot make, and the answer takes its colour
		// ask. Tap it for the owed colour, which floats restricted
		// (CR 106.6), exactly as a fixed restricted one above.
		anyColour := a.ManaColors == kop.ColAny
		if !restrSrcAb(a) || a.AFlags&kop.AfManaDynamic != 0 ||
			a.AFlags&kop.AfManaChoice != 0 && !anyColour || uint8(a.ManaColors)&want == 0 {
			continue
		}
		if anyColour {
			// The any-colour ability is a planner unit only for a payment
			// its RestrictValid$ admits (CR 106.6); the fixed ones above
			// keep the window's printed offer.
			d := s.payDescNow(NoObj)
			if !s.restrSrcAdmits(k, p, sl, a, &d) {
				continue
			}
		}
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		s.manaTapped(k, sl)
		if !s.addRestrChV(k, p, a.ManaRestrict, k.gen.restrVarOf(a), 0, bits.TrailingZeros8(uint8(a.ManaColors)&want), max(a.ManaN, 1)) {
			s.capFull(StallRestrSrc)
		}
		return true
	}
	return false
}

// censusGapMana: mana ability a is one gorge's planner census misses: a
// spend restriction, each-colour production, or a cost beyond {T} (and
// {T} plus sacrificing itself, a Treasure's last-resort tier).
func censusGapMana(a *kop.Ability) bool {
	if a.AFlags&kop.AfLatent != 0 {
		return false
	}
	f := a.Cost.Flags
	return a.ManaRestrict != 0 && !restrSrcAb(a) || a.AFlags&kop.AfManaEachColor != 0 || f != kop.CostTap && f != kop.CostTap|kop.CostSacSelf ||
		a.Cost.Mana.MV() != 0 || a.Cost.Mana.X != 0
}

// filtOnly: p's census gaps are filters alone (no other activatable source
// with a mana ability the census misses: a Bloom Tender, Giada, a
// Courtyard, Lupinflower Village's restricted {W}).
func (s *State) filtOnly(k *prep, p uint8) bool {
	return s.scriptFilt(k, p).any() && !s.gapOthers(k, p).any()
}

// gapOthers: p's activatable sources with a census gap that is no filter. A tap-another
// source short of what its cost taps (Baylen with one token) cannot be activated, so
// gorge's census does not miss it.
func (s *State) gapOthers(k *prep, p uint8) slotMask {
	m := s.gapMask(k, p, gfGapOther)
	if !k.tapMana {
		return m
	}
	live := s.scriptTapX(k, p)
	for b := m; b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		if ab := k.cards[c].manaAb; ab >= 0 && kop.TapXMana(&k.t.Cards[c].Abilities[ab]) && !live.has(sl) {
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
	// other: a census gap that is no filter (mixed gaps, zmixgaps file), or
	// an answer or a pass while a step's trigger resolves (act)
	other otherStep
	act   Action
}

type filtKey struct {
	pool [kop.NumManaKinds]uint8
	rb   [maxRB]rbatch // the restricted mana (State.rb)
	// art: the pool slots holding an artifact's mana (genState.poolArt):
	// gorge's key reads the typed units apart (Player.ManaUnits), so a
	// Three Tree Mascot's {W} over a Plains' {W} is a new state
	art    uint8
	tapped slotMask
	sp     uint8 // gorge's key counts the stack, hand and graveyard too (a Taps trigger's loot)
	hand   uint8
	gy     uint8
}

const maxFiltPath = 16

// maxFaceFilters bounds the filters faceReachable tries in combination.
const maxFaceFilters = 6

// filtSearch is gorge's prefix search (scriptSearch.visit) over filter gaps.
type filtSearch struct {
	a     Action
	p     uint8
	gaps  slotMask // the root census's filter gaps
	other slotMask // its gaps that are no filter (otherGap)
	fees  int      // the funding the search may activate
	own   Slot     // the source whose {T} is the play's own cost (NoSlot: none)
	sp0   uint8    // the stack at the root: only a deeper one is the path's to resolve
	nodes int
	nseen int
	seen  [potentialScriptNodes]filtKey
	n     int
	path  [maxFiltPath]filtStep
	// spill: a step of the path found floated restricted mana past what
	// State.rb holds (a restriction past restrKinds), unrestricted: the
	// script the search found is not one gorge's planner would price
	spill uint8
}

// filtScript reports whether gorge's prefix search finds a script for play
// a of p; with apply it plays the script on s (the play's prefix).
//
//go:noinline
func (s *State) filtScript(k *prep, a Action, p uint8, apply bool) bool {
	var fs filtSearch
	fs.a, fs.p, fs.gaps, fs.own, fs.sp0 = a, p, s.scriptFilt(k, p), NoSlot, s.sp
	fs.other = s.otherGaps(k, p)
	if a.Kind() == AkActivate {
		_, fs.own, _ = s.activationMana(k, Obj(a.Source()), a.Mode())
	}
	if s.filtRelaxable(k, fs.gaps) && !(k.tapMana && s.tapXTwoLive(k, p)) { // (unrelaxable: no funding)
		for b := fs.gaps; b.any(); b = b.rest() {
			fs.fees += s.filtFee(k, b.first())
		}
	}
	t := s.fork()
	defer s.unfork()
	t.g.noGrant = a.Kind() == AkActivate // the activation's granted mana is the grant search's
	if !fs.visit(k, t, 0, false) {
		return false
	}
	if fs.spill != 0 {
		s.capFull(fs.spill) // (the fork the step ran on latched it; the real state must)
	}
	if !apply && (k.restrSrc || fs.other.any()) && s.filtDeadEnd(k, p, a, &fs) {
		return false
	}
	if apply {
		s.filtPlay(k, p, fs.path[:fs.n], true)
	}
	return true
}

// filtPlay plays a found path on s: each activation as the search made it,
// each answer and pass as the decision then pending (the other player passes
// while the trigger resolves, tapXPlay). With apply the taps also fire the
// Taps triggers of a granted mana ability's source.
func (s *State) filtPlay(k *prep, p uint8, path []filtStep, apply bool) {
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
		if apply && (st.other.kind == otherNone || st.other.kind == otherGrant) && !(st.gap && st.ans > 0 && s.filtNoTap(k, st.sl)) {
			s.manaTapped(k, st.sl) // (a once-each-turn filter taps nothing)
		}
	}
	s.Decisions = n
}

// tick counts one visited node; false once the budget is spent.
func (fs *filtSearch) tick() bool {
	if fs.nodes >= potentialScriptNodes {
		return false
	}
	fs.nodes++
	return true
}

// filtDo plays step st for p.
func (s *State) filtDo(k *prep, p uint8, st filtStep) {
	if st.other.kind != otherNone {
		s.otherDo(k, p, st)
		return
	}
	pl := &s.pl[p&1]
	if st.gap && st.ans > 0 {
		s.filtUse(k, st.sl) // (a once-each-turn filter spends its use, not its {T})
	} else {
		s.perms[st.sl].flags |= pfTapped
		s.touchSl(st.sl)
		s.manaTapped(k, st.sl)
	}
	switch {
	case st.gap && st.ans > 0:
		// the fee is paid from the pool, generic in C W U B R G order, after
		// the restricted mana it may spend
		fee := s.filtFee(k, st.sl)
		fee -= s.filtFeeRestr(k, p, st.sl, fee)
		for _, c := range [6]int{kop.C, kop.W, kop.U, kop.B, kop.R, kop.G} {
			for fee > 0 && pl.pool[c] > 0 {
				pl.pool[c]--
				fee--
			}
		}
		if k.artMana {
			s.artSync(p) // a slot the fee emptied no longer holds an artifact's mana
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
		pl.pool[st.ans-1] += uint8(s.filtUnits(k, st.sl))
		// the filter taps the source for mana, so a TapsForMana static adds
		// its extra beside the filter's unit (mana_tapextra.go filtExtra)
		xv := s.filtExtra(k, st.sl)
		s.addXv(p, &xv)
		s.artTag(k, p, st.sl, int(st.ans-1)) // an artifact's mana is typed (Three Tree Mascot, Prophetic Prism; mana_artifact.go)
	case st.gap:
		pc := s.manaPCGen(k, st.sl)
		c := bits.TrailingZeros8(s.srcMaskAt(pc, st.sl))
		pl.pool[c] += max(pc.manaN, 1)
		s.addExtra(k, p, st.sl, pc)
		s.artTag(k, p, st.sl, c)
	case st.restr:
		s.filtRestrDo(k, p, st)
	default:
		pl.pool[st.ans] += max(s.manaPCGen(k, st.sl).manaN, 1)
		s.addExtra(k, p, st.sl, s.manaPCGen(k, st.sl))
		s.artTag(k, p, st.sl, int(st.ans))
	}
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
	c.filtDo(k, fs.p, st)
	fs.path[fs.n] = st
	fs.n++
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
	fs.n--
	return false
}

// visit is one priority node of the search.
func (fs *filtSearch) visit(k *prep, t *State, funded int, gapUsed bool) bool {
	if !fs.tick() {
		return false
	}
	p := fs.p
	key := filtKey{pool: t.pl[p&1].pool, rb: t.rb[p&1], sp: t.sp, hand: t.pl[p&1].handN, gy: t.pl[p&1].graveN}
	if k.artMana {
		t.artSync(p)
		key.art = t.g.poolArt[p&1]
	}
	for b := t.ctrlM[p&1].and(t.occ); b.any(); b = b.rest() {
		if sl := b.first(); t.perms[sl].flags&pfTapped != 0 {
			key.tapped = key.tapped.with(sl)
		}
	}
	for i := 0; i < fs.nseen; i++ {
		if fs.seen[i] == key {
			return false
		}
	}
	if fs.nseen < len(fs.seen) {
		fs.seen[fs.nseen] = key
		fs.nseen++
	}
	if gapUsed {
		var d Decision
		t.restrOptions(k, &d, p) // (restricted mana floating is priced as ordinary: the search reads no restriction)
		if hasOption(&d, fs.a) {
			return true
		}
	}
	var order [MaxPerm]Slot
	n := t.battlefieldOrder(p, t.ctrlM[p&1].and(t.occ), order[:], 0)
	for pass := 0; pass < 2; pass++ {
		gap := pass == 0
		if !gap && funded >= fs.fees {
			break
		}
		for i := 0; i < n; i++ {
			sl := order[i]
			if (fs.gaps.has(sl) || fs.other.has(sl)) != gap || sl == fs.own {
				continue // (tapped for mana, the play's source could not pay its own {T}: CR 602.2b)
			}
			if gap && fs.other.has(sl) {
				if fs.otherGap(k, t, sl, funded) {
					return true
				}
				continue
			}
			if gap && k.gen.anyFiltFirst {
				if !t.filtUsable(k, sl) {
					continue // (a once-each-turn filter: its use, not its {T})
				}
			} else if t.perms[sl].flags&pfTapped != 0 {
				continue
			}
			if gap {
				free := t.canTapForMana(k, sl)
				filt := poolTotal(&t.pl[p&1])+t.filtFeeRestrN(k, p, sl) >= t.filtFee(k, sl)
				if rm := t.mixSrcMask(k, sl); free && rm != 0 {
					if fs.mixSrcGap(k, t, sl, funded, rm, filt) { // (a restricted ability beside the filter: mana_mixsrc.go)
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
					if fs.try(k, t, filtStep{sl: sl, gap: true}, funded, true) {
						return true
					}
				case filt:
					if fs.filtColours(k, t, sl, funded) {
						return true
					}
				}
				continue
			}
			rm := uint8(0)
			if k.restrSrc {
				rm = t.filtRestrMask(k, sl)
			}
			free := t.canTapForMana(k, sl)
			if !free && rm == 0 {
				continue
			}
			if free && rm != 0 && !fs.tick() {
				return false // the ask between its census and its restricted ability
			}
			if free {
				pc := t.manaPCGen(k, sl)
				if pc.manaSac {
					continue // (a Treasure funds nothing here: see the handoff's open items)
				}
				mask := t.srcMaskAt(pc, sl)
				if bits.OnesCount8(mask) > 1 && !fs.tick() {
					return false // the colour ask
				}
				for m := mask; m != 0; m &= m - 1 {
					if fs.try(k, t, filtStep{sl: sl, ans: uint8(bits.TrailingZeros8(m))}, funded+1, gapUsed) {
						return true
					}
				}
			}
			if bits.OnesCount8(rm) > 1 && !fs.tick() {
				return false // the restricted ability's colour ask
			}
			for m := rm; m != 0; m &= m - 1 { // the restricted ability's answers, after the census one's
				if fs.try(k, t, filtStep{sl: sl, ans: uint8(bits.TrailingZeros8(m)), restr: true}, funded+1, gapUsed) {
					return true
				}
			}
		}
	}
	if t.sp > fs.sp0 && fs.n < maxFiltPath {
		// a trigger a step caused: let it resolve, the pool floats (gorge
		// scriptSearch.visit, the last resort at a node; a sorcery-speed play
		// waits for it, waitsForStack)
		pass := MakeAction(AkPass, 0, 0, 0)
		c := t.fork()
		c.Apply(pass)
		fs.path[fs.n] = filtStep{sl: NoSlot, other: otherStep{kind: otherPass}, act: pass}
		fs.n++
		ok := fs.settle(k, c, funded, true)
		t.unfork()
		if ok {
			return true
		}
		fs.n--
	}
	return false
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
		return false
	}
	if d.Kind == DkPriority {
		return fs.visit(k, t, funded, gapUsed)
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
func (fs *filtSearch) filtColours(k *prep, t *State, sl Slot, funded int) bool {
	if !fs.tick() {
		return false
	}
	if comboFiltAb(t.filtAb(k, sl)) {
		return fs.filtCombo(k, t, sl, funded)
	}
	var order [5]uint8
	n := manaColourOrder(t.filtAb(k, sl), &order)
	for i := 0; i < n; i++ {
		c := order[i]
		if fs.try(k, t, filtStep{sl: sl, ans: uint8(1 + c), gap: true}, funded, true) {
			return true
		}
	}
	return false
}

// filtCombo is the allocation ask of a combination filter at sl: gorge's
// scriptAnswers enumerates a Min == Max == n colour allocation unit by unit,
// the last unit varying fastest, and stops at 64 combinations. Each is one
// activation producing its counts (filtStep.alloc); the first whose
// production pays later is the script's.
func (fs *filtSearch) filtCombo(k *prep, t *State, sl Slot, funded int) bool {
	n := int(t.filtAb(k, sl).ManaN)
	if n <= 0 || n > 6 {
		return false
	}
	const per = 5 // Any's W U B R G, the order askManaColor offers
	var idx [6]uint8
	for tries := 0; tries < 64; tries++ {
		var st filtStep
		st.sl, st.ans, st.gap = sl, 1, true
		for u := 0; u < n; u++ {
			st.alloc[idx[u]]++
		}
		if fs.try(k, t, st, funded, true) {
			return true
		}
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
	return false
}
