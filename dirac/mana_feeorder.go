package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// feeOrder: gorge's feeOrder run (rules/potential_script.go
// potentialPrefixScript's second build with scriptSearch.feeOrder set). The
// plain run (the node-for-node mirror the offers are priced by) can spend its
// whole node budget on a fee-carrying gap's own abilities before any
// Baxter-untapped node whose pool pays the fee is reached, and then the
// phased run's feeOrder phases price the play: the covered funding sources
// first (phase 0, while funding lasts), then the gaps whose ability itself
// costs mana -- its fee paid from the floating pool (CR 601.2b) -- then the
// free gaps. potentialPrefixScript sets feeOrder on every plain cast of its
// printed cost whose census gaps include such a fee-carrying, multi-unit
// source (Baxter Building's "{4}, {T}: Add four mana in any combination of
// colours" funding Attuma, Atlantean Warlord), and scriptSearch.phasedScope
// admits the run's script for that cast, so the kernel must walk the same
// order to list the same casts.

// feeGapSlot reports whether the filter gap at sl carries a fee and makes more
// than one unit: gorge's scriptSearch.feeGaps (cc != nil, units >= 2, the
// cost's CMC or its X above zero). A one-unit fee source is a filter the
// fee-only paths price, and funding it first only changes the node walk
// (potential_script.go feeGaps).
func (s *State) feeGapSlot(k *prep, sl Slot) bool {
	if k.gen.filt(s.cardAt(sl)) < 0 {
		return false
	}
	return s.filtFee(k, sl) > 0 && s.filtUnits(k, sl) >= 2
}

// feeOrderEligible reports whether fs's failed plain run hands the search to
// the feeOrder run: the play is a plain cast of its printed cost (gorge's
// o.Kind == "cast", o.Mode == "", o.AltCostIndex == 0, playBaseCost's plain
// cast arm), a payment probe keeps the plain order it was asked for, the
// board has no other kind of gap (the kernel's filter route walks filters
// alone there; gorge's other gaps are its own file's subject) and the gaps
// include a fee-carrying, multi-unit filter. gorge's !phasedScope(e) is the
// rest of the gate: a cast whose own cost sacrifices or returns a permanent,
// or whose gaps include a hybrid fee, keeps the plain phased order (the
// consuming cost and the hybrid fee are the phased run's own scope), and
// those are the older routes here.
func (fs *filtSearch) feeOrderEligible(k *prep, t *State) bool {
	if fs.pay != nil || fs.census || fs.phased || fs.other.any() ||
		fs.a.Kind() != AkCast || !fs.plainCastOfPrintedCost(k, t) {
		return false
	}
	fee := false
	for m := fs.gaps; m.any(); m = m.rest() {
		sl := m.first()
		if _, _, hyb := t.filtHyb(k, sl); hyb {
			return false // a hybrid fee: the phased run's own scope, not feeOrder's
		}
		if t.feeGapSlot(k, sl) {
			fee = true
		}
	}
	return fee
}

// plainCastOfPrintedCost is playBaseCost's plain cast arm: the cast's printed
// cost, no may-play raise and no additional cost that consumes a permanent
// (gorge's phasedScope reads the base's Sac and Return parts: the activation
// comes before the sacrifice, CR 601.2b, and gorge keeps the phased order's
// own scope for that cast).
func (fs *filtSearch) plainCastOfPrintedCost(k *prep, t *State) bool {
	if _, _, ok := t.playBaseCost(k, fs.a); !ok {
		return false // playBaseCost's plain cast arm: a mode, a may-play or an alternative keeps the older routes
	}
	return spellAddCost(k, t.objs[fs.a.Source()].card).Flags&
		(kop.CostSac|kop.CostSacSelf|kop.CostReturn|kop.CostReturnSelf) == 0
}
