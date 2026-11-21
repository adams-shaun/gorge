package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// grantRelaxedAffords is gorge's relaxed unpayability proof
// (potential_plan.go paymentPlanRelaxedAlternatives) for an activation
// grantCap's walk lists: the census gaps become free alternatives, but a
// granted mana ability keeps its spend restriction, so a restricted grant the
// activation may not spend (Mm'menon's "{T}: Add {U}. Spend this mana only to
// cast a spell from anywhere other than your hand" under an Equip) is no unit
// of it. A play the relaxed census cannot pay is proven insufficient and takes
// no prefix search, however a granted tap would untap a land (Nanoform
// Sentinel). A Lotus Blossom's petals beside a Gift of Paradise's two mana
// pay Path to the World Tree's {2}{W}{U}{B}{R}{G} (tail-74 pair 49, decision
// 610). m is the walk's avail mode. A cast, or an activation with its own
// {T} source, keeps the walk's answer.
func (s *State) grantRelaxedAffords(k *prep, p uint8, a Action, cost *kop.ManaCost, ex Slot, d payDesc, m uint16) bool {
	if a.Kind() != AkActivate || ex != NoSlot {
		return true
	}
	var av avail
	av.xd, av.xdOn = d, true
	av.dynSac = true // a dynamic {T}, sacrifice source (Lotus Blossom) is a relaxed unit of its petals' mana: gorge's relaxed alternative takes its larger live amount, once
	s.g.availX = availPlain | m
	s.availFor(k, p, &av)
	s.g.availX = 0
	if k.tapMana {
		s.availTapX(k, p, &av) // the relaxed census counts a tap-another source as a free any-colour unit (mana_tapanother.go)
	}
	var filts [relaxedPaidLimit]Slot
	nf := 0
	if k.gen.anyFilt {
		if f := s.scriptFilt(k, p); s.filtRelaxable(k, f) {
			// the relaxed proof also appends each subset of the census's paid
			// alternatives, charging their fees (paymentPlanRelaxProof): Hidden
			// Grotto's {1},{T} makes the {U} a Forest board lacks, as
			// relaxRestrRefuses reads it
			for ; f.any(); f = f.rest() {
				filts[nf] = f.first()
				nf++
			}
		}
	}
	// the floating restricted mana the activation may spend is in the pool
	// the planner's verdict pays from (gorge AvailableFor; CR 106.6): Cargo
	// Ship's artifact-only {C} toward an Equipment's equip
	if s.rbAny(p) {
		if rm := s.restrPoolAct(k, p, &d); rm != 0 {
			s.restrAvailAdd(p, rm, &av)
		}
	}
	// a restricted planner source the activation may spend is a census unit
	// of its own (Jasmine Dragon Tea Shop's any colour for an Ally source's
	// ability, Merchant of Many Hats' {2}{B} from the graveyard; CR 106.6)
	// one run per subset of the paid alternatives (availFiltSubset): a
	// filter whose fee exceeds what it makes (Henge of Ramos's {2},{T}) only
	// costs mana, so the run that leaves it out is the one that pays
	for mask := 0; mask < 1<<nf; mask++ {
		run := av
		s.availFiltSubset(k, &run, filts[:nf], mask)
		if run.affords(cost, pipMaskOf(cost), 0) || k.restrSrc && s.restrSrcAffords(k, p, d, cost, &run, NoSlot, 0) {
			return true
		}
	}
	return false
}

// availFiltSubset adds to a the filters of f that mask selects, as availFilt
// adds every paid gap: a free unit of its colours, its fee off the total.
func (s *State) availFiltSubset(k *prep, a *avail, f []Slot, mask int) {
	for i, sl := range f {
		if mask>>i&1 == 1 {
			addFiltUnits(a, s.filtAb(k, sl), uint8(s.filtUnits(k, sl)))
			a.total -= s.filtFee(k, sl)
		}
	}
}

// grantJoinPays is the offer walk's own affordability read for the activation
// the grant search is about to re-price: the plain census (availFor, one unit
// per source -- the join of a source's own and granted abilities, availGrant)
// affording the composed cost is a payment real taps reach (CR 605.1a: one
// tap per source), and gorge's offer gate prices the play with the same
// census (legal_walk_autopay_fast.go autoPayFast's units) on a pool with no
// restricted batch -- the fast path's own first gate (CR 106.6: a restricted
// batch may not spend on the ability, so with one floating gorge prices the
// play through its planner, whose verdict the searches below own). So gorge's
// offer keeps it and the search's node budget is not spent re-proving what
// the census already priced -- a budget the search otherwise spends walking a
// wide any-colour board's tap orders and, spent, drops an offer gorge lists
// (tail-71-c000, seed 845794: Wakeroot Elemental's {G}{G}{G}{G}{G} beside
// Dune Chanter's nine any-colour granted taps). The census gaps the plain
// census leaves out -- a tap-another source, a printed sacrifice whose
// granted tap the join double-counts (mayPlayJoinCensusPays' two guards) --
// keep the search's answer.
func (s *State) grantJoinPays(k *prep, p uint8, cost *kop.ManaCost, d payDesc) bool {
	if cost.MV() == 0 || s.rbAny(p) || s.untappedTapGapLive(k, p) || s.ownSacTapNeeded(k, p, cost) {
		return false // gorge's fast path is off, or the prefix search prices these, not the census
	}
	var av avail
	av.xd, av.xdOn = d, true
	av.dynSac = true // a dynamic {T}, sacrifice source is one activation's unit
	s.availFor(k, p, &av)
	// the floating restricted mana the activation may spend is in the pool
	// the planner's verdict pays from (gorge AvailableFor; CR 106.6)
	if s.rbAny(p) {
		if rm := s.restrPoolAct(k, p, &d); rm != 0 {
			s.restrAvailAdd(p, rm, &av)
		}
	}
	return av.affords(cost, pipMaskOf(cost), 0) || k.restrSrc && s.restrSrcAffords(k, p, d, cost, &av, NoSlot, 0)
}

// manaLandTap is a debug helper (DIRAC_DBG_ASK): the battlefield mana
// lands' tapped flags, in slot order.
func (s *State) manaLandTap(k *prep, p uint8) string {
	out := ""
	m := s.ctrlM[p]
	for m.any() {
		sl := m.first()
		m = m.rest()
		if s.objs[sl].zone == kop.ZoneBattlefield && k.cards[s.cardAt(sl)].manaAb >= 0 {
			if s.perms[sl].flags&pfTapped != 0 {
				out += "T"
			} else {
				out += "u"
			}
		}
	}
	return out
}
