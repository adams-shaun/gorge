package kgen

import (
	"strconv"
	"strings"
)

// Dredge N (CR 702.52a): "If you would draw a card, you may mill N cards
// instead. If you do, return this card from your graveyard to your hand."
// gorge reads the printed number at every draw (engine/effects cardflow.go
// drawFor's dredge ask over dredgeCandidates), so kgen only records it:
// CardDef.Dredge, read by the kernel's draw paths (kw_dredge.go).

// lowerParamKeyword lowers the printed keywords with a parameter that have a
// lowering of their own, reporting whether head is one of them.
func (lc *lowerCtx) lowerParamKeyword(node, head, param string) bool {
	switch head {
	case "Bloodthirst":
		// CR 702.54a: gorge bloodthirstEntryMatch's synthetic Moved
		// replacement, lowered as gorge builds it (lower_bloodthirst.go)
		lc.guard(node, func() error { return lc.lowerBloodthirst(param) })
	case "Dredge":
		lc.lowerDredge(param) // CR 702.52
	default:
		return false
	}
	return true
}

// dredgeMax is the largest Dredge number lowered (the corpus prints 1..6).
const dredgeMax = 15

// lowerDredge lowers the printed K:Dredge:N; any N outside 1..dredgeMax, or a
// second Dredge line, refuses by name.
func (lc *lowerCtx) lowerDredge(param string) {
	n, err := strconv.Atoi(strings.TrimSpace(param))
	if err != nil || n < 1 || n > dredgeMax || lc.out.Dredge != 0 {
		lc.refuseNode("K:Dredge:"+param, "dredge %q", param)
		return
	}
	lc.out.Dredge = uint8(n)
}
