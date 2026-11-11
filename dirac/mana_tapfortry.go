package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// tapForTry is tapForRider for a cost the caller may find unpayable: it taps
// the plan and reports true, or taps nothing and reports false when the
// planner has none (tapForRider panics there, a kernel defect for a cost the
// offer priced). Only a cost the offer could not price yet, as a per-target
// raise charged after the targets (cost_raise.go), uses it.
func (s *State) tapForTry(k *prep, p uint8, cost *kop.ManaCost, x int, exclude Obj, anyColor bool) bool {
	var pk planner
	pk.try = true
	s.tapForPK(k, p, cost, x, exclude, false, anyColor, &pk)
	return pk.needT == 0 || pk.found
}
