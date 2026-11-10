package kgen

// exploreRememberedOne admits Explore Defined$ Remembered ("return target
// creature card ..., then it explores"). gorge effExplore explores every
// object of the resolution's remembered set; the kernel's OpExplore reads
// the set's first object (RefRemembered), so the shape lowers only behind
// exactly one remembered-set writer in the ability's own program that moves
// one object: a Return of a one-object target slot (ReturnEx MvRemember
// remembers the moved card, the new object, CR 400.7). An empty set (the
// return did not happen) explores nothing on both sides.
func (lc *lowerCtx) exploreRememberedOne() error {
	n := 0
	var w lInstr
	for _, in := range lc.ab.Prog {
		if kremWrites(in) {
			n++
			w = in
		}
	}
	if n != 1 || w.Op != "Return" || w.Obj.Kind != rTarget {
		return refuse("Explore of Remembered without one single-object Return writer")
	}
	if t := lc.ab.Targets[w.Obj.N]; !t.Max.isConst() || t.Max.N > 1 {
		return refuse("Explore of Remembered over a multi-object Return")
	}
	return nil
}
