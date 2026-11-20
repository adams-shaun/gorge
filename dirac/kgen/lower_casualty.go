package kgen

import (
	"strconv"
	"strings"
)

// lowerCasualty is lowerKeywords' K:Casualty arm.
func (lc *lowerCtx) lowerCasualty(node, param string) {
	// CR 702.153a: the optional creature sacrifice and the copy
	// trigger are the kernel's casualty cast (kw_casualty.go). The
	// fixed form only: Casualty:X (Ob Nixilis, the Adversary) and
	// its NonLegendary$ / SetLoyalty$ riders are refused.
	n, err := strconv.Atoi(param)
	switch {
	case param == "X" || strings.HasPrefix(param, "X:"):
		lc.refuseNode(node, "variable casualty (Casualty:X, its riders %q) is not lowered", param)
	case strings.Contains(param, ":"):
		lc.refuseNode(node, "casualty with riders %q (only a fixed Casualty:<N> is lowered)", param)
	case err != nil || n < 1 || n > 255:
		lc.refuseNode(node, "casualty amount %q", param)
	default:
		lc.out.Casualty = n
	}
}
