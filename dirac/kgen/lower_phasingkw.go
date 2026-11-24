package kgen

func init() { kwHandlers["Phasing"] = (*lowerCtx).lowerPhasingKw }

// lowerPhasingKw lowers a printed bare K:Phasing (CR 702.26a) to
// CardDef.Phasing, with no ability of its own: gorge expands nothing for it
// and reads the head in the untap step (rules/turn.go finishUntapStep, the
// phasing action before the untap action: a phased-in carrier phases out, a
// phased-out permanent phases in, each read once as the scan reaches it),
// which the kernel's untap-step phasing action reproduces
// (dirac/kw_phasing.go phaseUntap, read over the derived keywords: the
// printed keyword, none while face down or after losing all abilities). A
// granted Phasing refuses elsewhere; a second printed line is redundant
// (CR 702.26b-style duplicates do not stack: the keyword is read as present or
// absent) and lowers once.
func (lc *lowerCtx) lowerPhasingKw(node, param string) {
	if param != "" {
		lc.refuseNode(node, "phasing with a parameter %q", param)
		return
	}
	lc.out.Phasing = true
}
