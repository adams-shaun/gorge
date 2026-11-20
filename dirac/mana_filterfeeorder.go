package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The fee-order run of the filter search (gorge's scriptSearch.feeOrder,
// potential_script.go potentialPrefixScript's second build). When the plain
// run finds no script for a plain cast beside a multi-unit fee-carrying gap
// (Baxter Building's {4},{T}: four mana in any combination), gorge runs its
// phased search in the generic mana-fee order: the covered funding first, then
// the fee-carrying gaps (their fee is a mana payment settled from the floating
// pool, CR 601.2b, 605.3b), then the other gaps. The plain order spends its
// 160 nodes on the colour branches of the other sources before the funding
// floats, so a line the fee-order run reaches in a dozen nodes is out of its
// budget. The run is a fallback only: a play the plain run priced keeps its
// script.

// feeOrderGaps is the filters of gaps whose ability costs mana and makes more
// than one unit (gorge's feeGaps): a one-unit filter nets nothing after its
// fee, and funding it first only changes the node walk. A dynamic amount
// counts no unit there (availableAmount prices only a literal one), so Three
// Tree City and Starfleet Academy are no fee gaps whatever they make.
func (s *State) feeOrderGaps(k *prep, gaps slotMask) slotMask {
	var m slotMask
	for ; gaps.any(); gaps = gaps.rest() {
		if sl := gaps.first(); s.filtAb(k, sl).AFlags&kop.AfManaDynamic == 0 && s.filtFee(k, sl) > 0 && s.filtUnits(k, sl) >= 2 {
			m = m.with(sl)
		}
	}
	return m
}

// feeOrderRun reports whether gorge runs the fee-order search for the play of
// fs: a plain cast of the printed cost (not an alternative, may-play, face or
// warp one) with a fee-carrying multi-unit gap, outside phasedScope (a cost
// that sacrifices or returns a permanent, or a hybrid-fee gap, takes gorge's
// ordinary phased order, which this search does not model). A payment probe
// (filtPay) and a board with a filter that is its card's first ability (its
// gate reads the floating pool) keep the plain run's answer.
func (s *State) feeOrderRun(k *prep, a Action, fs *filtSearch) bool {
	if fs.pay != nil || k.gen.anyFiltFirst || a.Kind() != AkCast || a.Mode()&3 != 0 || a.Mode()&(castMayPlay|castFace|castWarp) != 0 {
		return false
	}
	if !s.feeOrderGaps(k, fs.gaps).any() || s.phasedScopeCost(k, a) {
		return false
	}
	for g := fs.gaps; g.any(); g = g.rest() {
		if _, _, hyb := s.filtHyb(k, g.first()); hyb {
			return false
		}
	}
	return true
}
