package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The fee-order fallback run (gorge scriptSearch.feeOrder, potentialPrefixScript's
// second build): a plain cast whose census holds a multi-unit fee-carrying filter
// (Baxter Building's {4},{T}: four mana in any combination) runs, when the plain
// search found no script, a phased search that taps the covered funding sources
// first, then the fee-carrying gaps, then the fee-free ones. The fee is paid from
// the floating pool (CR 601.2b), so the funding must float before the filter is
// activated; the plain order activates the gaps first and spends its node budget
// on the branches where the filter's fee is still unpaid.

// filtFeeGap: the filter at sl makes two or more mana for a mana fee (gorge's
// feeGaps: a one-unit filter is priced by the fee-only paths and funding it
// first only changes the node walk). A dynamic amount (Three Tree City's {2},{T})
// is no fee gap: gorge's availableAmount reads a non-literal Amount$ as zero.
func (s *State) filtFeeGap(k *prep, sl Slot) bool {
	return s.filtAb(k, sl).AFlags&kop.AfManaDynamic == 0 && s.filtUnits(k, sl) >= 2 && s.filtFee(k, sl) > 0
}

// feeOrderScope reports whether gorge's fallback run for fs's play is the
// fee order: a plain cast (no alternative or additional cost that sacrifices or
// returns a permanent, whose phased scope is another order) with a fee-carrying
// multi-unit filter among the gaps and no hybrid-fee one. A board with a gap that
// is no filter keeps the plain run's answer.
func (fs *filtSearch) feeOrderScope(k *prep, s *State) bool {
	a := fs.a
	if a.Kind() != AkCast || a.Mode()&3 != 0 || a.Mode()&(castMayPlay|castFace|castWarp) != 0 ||
		fs.pay != nil || fs.other.any() || s.filtHybIn(k, fs.gaps) {
		return false
	}
	if ac := spellAddCost(k, s.objs[Obj(a.Source())].card); ac.Flags&(kop.CostSac|kop.CostSacSelf|kop.CostReturn|kop.CostReturnSelf) != 0 {
		return false
	}
	for b := fs.gaps; b.any(); b = b.rest() {
		if s.filtFeeGap(k, b.first()) {
			return true
		}
	}
	return false
}

// feeOrderShape is visit's pass layout under the fee order: pass 0 the funding
// sources (skipped once the funding is spent: gorge continues, it does not
// break), pass 1 the fee-carrying gaps, pass 2 the fee-free gaps.
func (fs *filtSearch) feeOrderShape(pass, funded int) (gap, skip bool) {
	return pass > 0, pass == 0 && funded >= fs.fees
}
