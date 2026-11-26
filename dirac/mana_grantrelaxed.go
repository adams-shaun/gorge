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
// {T} source, keeps the walk's answer. So does a live filter the proof
// cannot relax (a hybrid fee's, Cascade Bluffs' {U/R},{T}; a repeatable
// one): gorge's census is unrelaxable there (paymentPlanRelaxedAlternatives
// ok=false) and its planner never runs the relaxed proof — an unrelaxable
// census falls through to the prefix search, whose script prices the play
// (potentialCensusFallback) — so the drop would be the search's to make.
func (s *State) grantRelaxedAffords(k *prep, p uint8, a Action, cost *kop.ManaCost, ex Slot, d payDesc, m uint16) bool {
	if a.Kind() != AkActivate || ex != NoSlot {
		return true
	}
	if k.gen.anyComboAny && s.reflAmtUnbounded(k, s.caGaps(k, p)) {
		// a live reflected Amount$ source (Incubation Druid) leaves gorge's census
		// unrelaxable too (mana_reflamt.go), so the proof never runs: the prefix
		// search prices the activation
		return true
	}
	var av avail
	av.xd, av.xdOn = d, true
	av.dynSac = true // a dynamic {T}, sacrifice source (Lotus Blossom) is a relaxed unit of its petals' mana: gorge's relaxed alternative takes its larger live amount, once
	s.g.availX = availPlain | m
	s.availFor(k, p, &av)
	s.g.availX = 0
	s.relaxedTapAnyColUnits(k, p, &av)
	if k.tapMana {
		s.availTapX(k, p, &av) // the relaxed census counts a tap-another source as a free any-colour unit (mana_tapanother.go)
	}
	var filts [relaxedPaidLimit]Slot
	nf := 0
	if k.gen.anyFilt {
		f := s.scriptFilt(k, p)
		if f.any() && !s.filtRelaxable(k, f) {
			return true
		}
		// the relaxed proof also appends each subset of the census's paid
		// alternatives, charging their fees (paymentPlanRelaxProof): Hidden
		// Grotto's {1},{T} makes the {U} a Forest board lacks, as
		// relaxRestrRefuses reads it
		for ; f.any(); f = f.rest() {
			filts[nf] = f.first()
			nf++
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
