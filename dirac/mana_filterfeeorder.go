package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The fee-order fallback (gorge potentialPrefixScript's second build with
// scriptSearch.feeOrder). When the plain run of a plain cast's prefix search
// spends its whole node budget without a script, gorge runs it again with the
// covered funding first, then the census gaps whose mana ability costs mana
// and makes more than one unit (Baxter Building's {4},{T}: four mana in any
// combination), then the other gaps: the fee is paid from mana already
// floating (CR 601.2b, 605.3b), so floating it first reaches the funding line
// the plain order's twin branches spent the budget on. A one-unit fee source
// is a filter whose net production is zero, which the plain order prices.
// An activated ability takes the same fallback (gorge builds the fee-order run
// for optAbility with no cast-cost or hybrid scope).

// filtFeeGaps is the filters of f gorge counts as fee gaps: a fee after cost
// reductions, and a fixed amount of at least two units (feeGaps in
// potentialPrefixScript; a dynamic amount counts none).
func (s *State) filtFeeGaps(k *prep, f slotMask) slotMask {
	var m slotMask
	for ; f.any(); f = f.rest() {
		sl := f.first()
		if s.filtAb(k, sl).AFlags&kop.AfManaDynamic == 0 && s.filtFee(k, sl) > 0 && s.filtUnits(k, sl) >= 2 {
			m = m.with(sl)
		}
	}
	return m
}

// feeOrderFallback reports whether gorge's fee-order run follows the plain run
// fs, which found no script: a plain cast of the printed cost (or an ability activation) on a board of
// filter gaps alone, one of them a fee gap, whose own cost sacrifices or
// returns nothing and whose gaps hold no hybrid fee on any mana ability of
// their faces (gapFaceHybIn; those are the phased order's own scope, gorge
// phasedScope). The plain run's budget must have run out: a completed search
// saw every state the reordered one reaches. It records the fee gaps on fs —
// only on success, so a declined fallback leaves the plain run's search as it
// was (every read is gated on fs.feeOrder, but the field is the fallback's to
// set).
func (s *State) feeOrderFallback(k *prep, p uint8, a Action, fs *filtSearch) bool {
	act := a.Kind() == AkActivate
	if fs.pay != nil || fs.other.any() || fs.nodes < potentialScriptNodes || (!act && (a.Kind() != AkCast || a.Mode() != 0)) {
		return false
	}
	feeGaps := s.filtFeeGaps(k, fs.gaps)
	if !feeGaps.any() {
		return false
	}
	if !act {
		// an activation's fee-order run is not scoped by phasedScope (it
		// reads a cast's cost and the gaps' hybrid fees only for a cast)
		if s.gapFaceHybIn(k, fs.gaps) {
			return false
		}
		o := Obj(a.Source())
		if o == NoObj || int(o) >= len(s.objs) {
			return false
		}
		ac := spellAddCost(k, s.objs[o].card)
		if ac.Flags&(kop.CostSac|kop.CostSacSelf|kop.CostReturn|kop.CostReturnSelf) != 0 {
			return false
		}
	}
	fs.feeGaps = feeGaps
	return true
}
