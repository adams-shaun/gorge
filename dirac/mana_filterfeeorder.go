package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The fee order: gorge's phased fallback run for a plain cast beside a
// multi-unit fee gap (rules/potential_script.go scriptSearch.feeOrder,
// potentialPrefixScript). Where the plain run spent its node budget and found
// no script, gorge runs the search again with the covered funding first, then
// the gaps whose ability makes more than one mana for a fee (Baxter Building's
// {4},{T}: four mana of any combination), then the other gaps, each gap
// gated on a fee the pool so far pays (scriptSearch.payable, CR 601.2b). The
// plain order spends its nodes on the free gap's branches before any fee is
// funded; this order reaches the payment in a handful (a Villainous Hideout
// beside the Forests has no twin, so the plain walk never finishes).
//
// Scope, as gorge's: a cast (no mode, no alternative cost) whose own cost
// sacrifices or returns nothing and whose gaps hold no hybrid fee (those
// are gorge's phasedScope, the phased order dirac does not model), on a
// filter-only board with no fee-only repeatable filter (phasedFallback's).

// castSacOrReturn: cast a's face costs a sacrifice or a return (gorge's
// phasedScope, mirrored by grantSrch.phasedScope).
func (s *State) castSacOrReturn(k *prep, a Action) bool {
	cd := &k.t.Cards[s.castFaceCard(k, a)]
	const sacOrReturn = kop.CostSac | kop.CostSacSelf | kop.CostReturn | kop.CostReturnSelf
	if cd.SpellCost.Flags&sacOrReturn != 0 || cd.AltAddCost.Flags&sacOrReturn != 0 {
		return true
	}
	for ai := range cd.Abilities {
		if ab := &cd.Abilities[ai]; ab.Kind == kop.AbSpell && ab.Cost.Flags&sacOrReturn != 0 {
			return true
		}
	}
	return false
}

// feeGap: the filter at sl costs mana and makes more than one unit (gorge's
// scriptSearch.feeGaps): a one-unit filter nets nothing after its fee, so the
// fee order leaves it with the other gaps. gorge counts a literal amount
// (availableAmount): a dynamic one (Starfleet Academy's creature types) is no
// units there, so it is no fee gap.
func (s *State) feeGap(k *prep, sl Slot) bool {
	return s.filtAb(k, sl).AFlags&kop.AfManaDynamic == 0 && s.filtFee(k, sl) > 0 && s.filtUnits(k, sl) >= 2
}

// feeOrderScope reports whether gorge's fee-order fallback prices play a, the
// plain run having found no script.
func (fs *filtSearch) feeOrderScope(k *prep, s *State, a Action) bool {
	if fs.pay != nil || fs.feeOnly || fs.other.any() || a.Kind() != AkCast || a.Mode() != 0 || s.castSacOrReturn(k, a) || s.filtHybIn(k, fs.gaps) {
		return false
	}
	for b := fs.gaps; b.any(); b = b.rest() {
		if s.feeGap(k, b.first()) {
			return true
		}
	}
	return false
}

// feeOrderRun is the fallback search for the play of fs.
func (fs *filtSearch) feeOrderRun() filtSearch {
	return filtSearch{a: fs.a, p: fs.p, gaps: fs.gaps, own: fs.own, sp0: fs.sp0, need: fs.need, useful: fs.useful,
		usefulOK: fs.usefulOK, other: fs.other, fees: fs.fees, dead: fs.dead, feeOrder: true}
}

// feeOrderPassShape is passShape for the fee order's passes: 0 the funding
// (skipped once it floated), 1 the multi-unit fee gaps, 2 the other gaps (3 is
// the life pass).
func (fs *filtSearch) feeOrderPassShape(pass, funded int) (gap, skip bool) {
	fs.feePass = uint8(pass)
	gap = pass != 0
	return gap, !gap && funded >= fs.fees
}

// feeOrderSkip: gap sl is not this fee-order pass's, or its fee is more than
// the pool holds (gorge's payable gate).
func (fs *filtSearch) feeOrderSkip(k *prep, t *State, sl Slot) bool {
	if !fs.feeOrder {
		return false
	}
	if !fs.gaps.has(sl) {
		return fs.feePass == 1
	}
	if t.feeGap(k, sl) != (fs.feePass == 1) {
		return true
	}
	return poolTotal(&t.pl[SeatIx(fs.p)]) < t.filtFee(k, sl)
}
