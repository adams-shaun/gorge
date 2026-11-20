package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// feeOrderGaps is the fee-carrying gaps of a plain cast's prefix search when
// gorge's phased run takes the generic mana-fee order (potentialPrefixScript:
// build(true) sets scriptSearch.feeOrder for an unmoded, non-alternative cast
// beside a census gap whose mana ability costs mana and makes two or more
// mana -- Baxter Building's {4},{T}: four mana -- unless phasedScope already
// puts the run in scope, a cast that sacrifices or returns a permanent or a
// hybrid-fee gap). The search it runs after the plain one found no script
// walks the covered funding first, then those gaps, then the others
// (filtSearch.feeOrder). The mask is empty when the run does not apply.
func (s *State) feeOrderGaps(k *prep, fs *filtSearch) slotMask {
	if fs.a.Kind() != AkCast || fs.a.Mode() != 0 || fs.pay != nil || !fs.gaps.any() {
		return slotMask{}
	}
	if ac := spellAddCost(k, s.objs[Obj(fs.a.Source())].card); ac.Flags&(kop.CostSac|kop.CostReturn) != 0 {
		return slotMask{} // phasedScope: the cast's own cost sacrifices or returns a permanent
	}
	var m slotMask
	for b := fs.gaps; b.any(); b = b.rest() {
		sl := b.first()
		if kop.HybridFilterManaAb(s.filtAb(k, sl)) {
			return slotMask{} // phasedScope: a hybrid-fee gap
		}
		// gorge counts a literal Amount$ (availableAmount); a dynamic one is 0 there
		if a := s.filtAb(k, sl); a.AFlags&kop.AfManaDynamic == 0 && s.filtUnits(k, sl) >= 2 {
			m = m.or(slotMask{}.with(sl))
		}
	}
	return m
}
