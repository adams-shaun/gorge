package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// anyColorWitness mirrors gorge's witness for a may-play cast under the
// MayPlayIgnoreColor rider (rules potential_plan.go potentialPinnedColourPlan).
// The planner prices every coloured pip generic (pay.PlanManaUnderRider), so
// its plan may tap a colourless-only source, and the offer gate then rejects
// the plan's pool: "any color" never includes colourless (CR 107.4c). The
// witness is the folded plan when its taps make at least one coloured unit per
// coloured pip, else the first of five re-plans that hold ALL the coloured
// pips on one colour (pay.PlanManaUnderRiderPinned) -- a cast whose coloured
// pips need two colours' sources and whose folded plan took the colourless
// one has no witness, so gorge leaves it out of the autopay offers (unpriced).
// cost is the printed cost, folded the planner's (planUnderRider).
func (s *State) anyColorWitness(k *prep, p uint8, exclude Obj, cost, folded kop.ManaCost, av *avail) bool {
	nCol := 0
	for c := 0; c < kop.C; c++ {
		nCol += int(cost.Pips[c])
	}
	pl := &s.pl[p]
	pooled := false
	for c := range pl.pool {
		pooled = pooled || pl.pool[c] != 0
	}
	if nCol == 0 || cost.Hyb[0] != 0 || cost.X != 0 || pooled {
		return true // no coloured pip, or a shape the fold's witness does not read the same way
	}
	pk := planner{dry: true}
	s.tapForPK(k, p, &folded, 0, exclude, false, false, &pk)
	if pk.found {
		col := 0
		for i := 0; i < pk.bestN; i++ {
			if ty := &pk.types[pk.bestTy[i]]; ty.mask&0x1f != 0 {
				col += int(ty.n)
			}
		}
		if col >= nCol {
			return true
		}
	}
	for c := 0; c < kop.C; c++ {
		pin := folded
		pin.Generic -= uint8(nCol)
		pin.Pips[c] = uint8(nCol)
		if av.affords(&pin, pipMaskOf(&pin), 0) {
			return true
		}
	}
	return false
}
