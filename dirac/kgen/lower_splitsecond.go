package kgen

// lowerSplitSecond lowers a printed K:Split second (CR 702.61a) to
// CardDef.SplitSecond and nothing else: gorge reads the derived keyword off
// each spell on the stack at every priority offer (rules split_second.go
// filterSplitSecondActions), and the kernel reads the card's flag off each
// spell entry, a copy included (CR 707.2), beside the StatStackKw grant
// (dirac/kw_splitsecond.go). Multiple instances are redundant (CR 702.61b).
func (lc *lowerCtx) lowerSplitSecond(node, param string) {
	if param != "" {
		lc.refuseNode(node, "split second with a parameter %q", param)
		return
	}
	lc.out.SplitSecond = true
}
