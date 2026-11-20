package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The fee-order phased run (gorge rules/potential_script.go: scriptSearch.feeOrder,
// phaseAt). A plain cast beside a census gap whose mana ability costs mana
// and makes two or more units (Baxter Building's {4},{T}: four mana in any
// combination) is priced by the plain run first, which walks the gaps before
// the funding: the filter's fee is paid from the floating pool (CR 601.2b),
// so its activation at an empty pool is no play, and the plain run spends its
// node budget on dead branches. Where it finds no script, gorge's phased run
// walks the covered funding first, then the multi-unit fee gaps, then the
// other gaps (phased order with feeOrder set).

// feeOrderGaps: the fee gaps of a filter-only search fs for play a, when gorge
// runs the feeOrder phased fallback for it: a plain cast of the printed cost
// (mode 0, no consuming additional cost), at least one fee gap, and no
// hybrid-fee gap (the hybrid filter's phased run is in gorge's own scope, not
// the feeOrder's).
func (s *State) feeOrderGaps(k *prep, a Action, p uint8, fs *filtSearch) (slotMask, bool) {
	if a.Kind() != AkCast || a.Mode() != 0 || fs.pay != nil {
		return slotMask{}, false
	}
	if sp := k.t.Cards[s.objs[Obj(a.Source())].card].SpellCost; sp.Flags != 0 || sp.N != 0 || sp.Ctr != 0 {
		return slotMask{}, false
	}
	var fg slotMask
	for b := fs.gaps; b.any(); b = b.rest() {
		sl := b.first()
		ab := s.filtAb(k, sl)
		if kop.HybridFilterManaAb(ab) {
			return slotMask{}, false
		}
		// a non-literal amount is zero units to gorge's availableAmount (the
		// dynamic Three Tree City filter is no fee gap)
		if ab.AFlags&kop.AfManaDynamic == 0 && ab.ManaN >= 2 {
			fg = fg.with(sl)
		}
	}
	return fg, fg.any()
}

// feeOrderShape is passShape's shape for the feeOrder run: pass 0 the covered
// funding (skipped once it floated: gorge's loop continues to the fee gaps),
// pass 1 the fee gaps, pass 2 the other gaps (skipSlot splits them), pass 3
// the life-only source.
func (fs *filtSearch) feeOrderShape(pass, funded int) (gap, freeOnly, skip bool) {
	if pass == 0 {
		return false, false, funded >= fs.fees
	}
	return true, false, false
}
