package kgen

import (
	"strings"
)

// lower_morphreduce.go: a face-down keyword whose turn-up cost carries a
// printed reduction ("Disguise {5}{R}. This cost is reduced by {1} for each
// ..."). The keyword's second colon field names the amount (an SVar or a
// literal); the third is the reminder text. gorge (rules morph_turnup.go
// morphFaceUpCost, morphTurnUpMods) folds the amount into the turn-up price
// as a generic reduction, floored at zero, at the offer, the re-proof and
// the settlement alike; the kernel subtracts CardDef.MorphReduce in its one
// turn-up price (dirac/face_morphreduce.go).

// morphReduceParam splits a Morph/Megamorph/Disguise parameter into its cost
// and its reduction amount: the plain cost when there is no colon field, else
// the cost and the lowered amount of a cost:amount:text parameter. ok=false
// is any other shape, or an amount that does not lower.
func (lc *lowerCtx) morphReduceParam(param string) (cost string, red lValue, ok bool) {
	parts := strings.SplitN(param, ":", 3)
	if len(parts) == 1 {
		return param, lValue{}, true
	}
	if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" || lc.svarReadsTarget(strings.TrimSpace(parts[1]), 0) {
		return "", lValue{}, false
	}
	v, err := lc.value(parts[1])
	if err != nil || v.Kind == vConst && v.N <= 0 {
		// gorge degrades an unresolvable amount to no reduction; the kernel
		// would read a different price, so the card refuses
		return "", lValue{}, false
	}
	return strings.TrimSpace(parts[0]), v, true
}
