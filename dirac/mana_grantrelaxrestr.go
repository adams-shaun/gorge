package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// grantrelaxrestr: the relaxed census's spend restrictions.
//
// gorge's unpayability proof (potential_plan.go paymentPlanRelaxProof)
// relaxes each granted mana ability to a free alternative, but the planner
// still refuses an alternative whose RestrictValid$ the payment does not
// admit (pay/plancost.go restrictedRefused). A play the relaxed census pays
// only with such a grant (Mm'menon's Spell.!wasCastFromYourHand {U} beside an
// Equip activation) is proven insufficient and never searched, though a tap
// of the granted source could fire a trigger that untaps a land (Nanoform
// Sentinel): the prefix search that would find that line does not run (CR
// 106.6, 605.3b: the triggered untap waits for the stack, so the activation
// is not payable now). grantCap's walk counts every grant (availWalk); this
// re-reads the play with each restricted grant its payment refuses dropped.

// relaxRestrRefuses reports whether the relaxed census pays cost for p only
// through a restricted granted ability the payment d may not spend.
//
//go:noinline
func (s *State) relaxRestrRefuses(k *prep, p uint8, cost *kop.ManaCost, d payDesc) bool {
	if k.gen == nil || !k.gen.anyGrantRestr() {
		return false
	}
	pm := pipMaskOf(cost)
	prev := s.g.availX
	s.g.availX = availPlain | availWalk
	var all, kept avail
	s.availFor(k, SeatIx(p), &all)
	kept.xd, kept.xdOn = d, true
	s.availFor(k, SeatIx(p), &kept)
	s.g.availX = prev
	s.availDepleteRiders(k, p, &all)
	s.availDepleteRiders(k, p, &kept)
	if k.tapMana {
		// a tap-another source is a free any-colour unit of the relaxed census
		// either way (mana_tapanother.go): it makes up what the grant lacks
		s.availTapX(k, SeatIx(p), &all)
		s.availTapX(k, SeatIx(p), &kept)
	}
	relax := k.gen.anyFilt && s.filtRelaxable(k, s.scriptFilt(k, p))
	subsets := 1
	if relax {
		// gorge's relaxed proof also appends each subset of the census's paid
		// alternatives, charging the subset's fees (paymentPlanRelaxProof): a
		// filter makes up the colour a refused grant lacks, in both reads. A
		// net-negative filter (Three Tree City's {2},{T} making one mana) is
		// left out of the subsets that pay without it (mana_filtersubset.go),
		// and the play is refused only when no subset's kept read pays.
		subsets = 1 << s.filtNetNegative(k, s.scriptFilt(k, p))
	}
	anyAll := false
	for m := 0; m < subsets; m++ {
		a, kp := all, kept
		if relax {
			s.g.availX |= availGaps // availFilt honours filtSkip only in the gaps walk
			s.setFiltSkip(uint8(m))
			s.availFilt(k, p, &a)
			s.availFilt(k, p, &kp)
			s.setFiltSkip(0)
			s.g.availX = prev
		}
		if !a.affords(cost, pm, 0) {
			continue
		}
		anyAll = true
		if s.relaxKeptPays(k, p, cost, pm, d, &kp) {
			return false
		}
	}
	return anyAll
}

// relaxKeptPays is relaxRestrRefuses's kept read: the census without the
// restricted grants d's payment refuses pays cost.
func (s *State) relaxKeptPays(k *prep, p uint8, cost *kop.ManaCost, pm uint8, d payDesc, kept *avail) bool {
	if k.restrSrc {
		// the restricted planner units the payment may spend (the Artificer's
		// "{T}: Add {C}. Spend this mana only to activate an ability or cast an
		// artifact spell" beside a grant that adds a refused any-colour ability
		// to it) are units of gorge's census per payment (pay/plancost.go
		// RestrictValidMatches): they stay in the kept read, so a refused grant
		// that only duplicates one of them leaves the play relaxed-payable
		u, multi, nm := s.restrSrcAvailD(k, p, d, kept, NoSlot, false, nil, 0)
		return u.affordsMulti(cost, 0, multi[:nm])
	}
	return kept.affords(cost, pm, 0)
}

// anyGrantRestr: some granted mana ability of the table carries a spend
// restriction.
func (g *genPrep) anyGrantRestr() bool {
	for _, r := range g.grantRestr {
		if r != 0 {
			return true
		}
	}
	return false
}

// grantRefusedPrices reports whether the walk prices play a of p only through
// a restricted granted mana ability a's payment may not spend (Clement's
// creature-spell-only {G} or {U} beside an instant): the walk counts every
// grant as a unit, but gorge's planner reads no granted ability, and the mana
// a prefix tap of that grant floats is refused by the payment (CR 106.6), so
// no node of gorge's search prices the play with it.
func (s *State) grantRefusedPrices(k *prep, p uint8, a Action) bool {
	if k.gen == nil || !k.gen.anyGrantRestr() || !isCastKind(a.Kind()) && a.Kind() != AkActivate {
		return false
	}
	cost, _, _, ok := s.grantPlayCost(k, p, a, true)
	return ok && s.relaxRestrRefuses(k, p, &cost, s.grantPayDesc(k, a))
}

// availDepleteRiders adds to av the production of p's live depletion riders
// (Sandstone Needle's {R}{R}, Remote Farm's {W}{W}) that no plain unit of the
// census holds. The relaxed proof counts such a source's whole production
// while it keeps the counter its cost removes (potentialGapCost), whichever
// granted ability the payment refuses, so a play the printed rider and the
// unrefused sources pay is no insufficient one: gorge's prefix search taps the
// rider and offers it (Rathi Intimidator's {2} over a Swamp, a Needle and a
// Food token holding a restricted granted ability; CR 605.3b, 601.2g).
func (s *State) availDepleteRiders(k *prep, p uint8, av *avail) {
	if k.gen == nil || !k.gen.anyManaRider {
		return
	}
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		sl := m.first()
		if !s.depleteCensusGap(k, p, sl) {
			continue
		}
		if a, ok := s.riderAbilityOf(k, sl); ok && !a.ManaEach {
			cols, n := tapXUnit(a)
			av.addUnits(cols&0x3f, n)
		}
	}
}
