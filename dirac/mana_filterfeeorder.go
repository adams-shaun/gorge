package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The phased run's generic mana-fee order (gorge scriptSearch.feeOrder,
// potentialPrefixScript's fallback build): a plain cast whose census gaps hold
// a multi-unit fee gap (Baxter Building's "{4}, {T}: Add four mana in any
// combination of colors") that the plain order's budget could not price walks
// the covered funding first, then the fee-carrying gaps, then the fee-free
// gaps. The fee a gap owes is paid from the floating pool (CR 601.2b), so
// funding first leaves it its mana, and the plain order spends its 160 nodes
// on the colour asks of the sources it taps before the fee is funded.

// filtFeeGaps: the filters of f whose mana ability costs mana and makes more
// than one unit (scriptSearch.feeGaps). A one-unit fee source nets zero, so
// its fee-only paths price it and funding it first only changes the walk.
func (s *State) filtFeeGaps(k *prep, f slotMask) slotMask {
	var m slotMask
	for ; f.any(); f = f.rest() {
		sl := f.first()
		a := s.filtAb(k, sl)
		// a dynamic amount is no literal: gorge's availableAmount reads it as zero
		if a.AFlags&kop.AfManaDynamic == 0 && (a.Cost.Mana.MV() > 0 || a.Cost.Mana.X != 0) && s.filtUnits(k, sl) >= 2 {
			m = m.or(slotBit(sl))
		}
	}
	return m
}

// feeOrderRun reports whether gorge's phased run takes the fee order for play
// a over the filter gaps of fs (potentialPrefixScript: a plain cast with a fee
// gap that no hybrid fee puts in the sacrifice-or-hybrid scope), and the fee
// gaps. A board with a gap that is no filter keeps the plain run's answer.
func (s *State) feeOrderRun(k *prep, a Action, fs *filtSearch) (slotMask, bool) {
	if a.Kind() != AkCast || a.Mode() != 0 || fs.other.any() || fs.pay != nil || s.filtHybIn(k, fs.gaps) {
		return slotMask{}, false
	}
	fg := s.filtFeeGaps(k, fs.gaps)
	return fg, fg.any()
}
