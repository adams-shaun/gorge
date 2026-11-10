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
	s.availFor(k, p&1, &all)
	kept.xd, kept.xdOn = d, true
	s.availFor(k, p&1, &kept)
	s.g.availX = prev
	if k.tapMana {
		// a tap-another source is a free any-colour unit of the relaxed census
		// either way (mana_tapanother.go): it makes up what the grant lacks
		s.availTapX(k, p&1, &all)
		s.availTapX(k, p&1, &kept)
	}
	if k.gen.anyFilt && s.filtRelaxable(k, s.scriptFilt(k, p)) {
		// gorge's relaxed proof also appends each subset of the census's paid
		// alternatives, charging the subset's fees (paymentPlanRelaxProof): a
		// filter makes up the colour a refused grant lacks, in both reads
		s.availFilt(k, p, &all)
		s.availFilt(k, p, &kept)
	}
	return all.affords(cost, pm, 0) && !kept.affords(cost, pm, 0)
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
