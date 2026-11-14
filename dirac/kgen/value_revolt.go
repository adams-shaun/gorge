package kgen

import "strings"

// revoltValue lowers Count$Revolt.<yes>.<no> (gorge count_body_heads.go
// evalCountBodyDottedRevolt, CR 702.38): <yes> when a permanent the resolving
// controller controlled left the battlefield this turn, else <no>; each arm a
// literal or an SVar (gorge dotBranch). The predicate is the one the bare
// Condition$ Revolt gate and the Revolt$ clauses read (vRevolt, kernel
// revoltHolds). Fatal Push's Count$Revolt.4.2 destroy bound, Lifecraft
// Cavalry's Count$Revolt.1.0.
func (lc *lowerCtx) revoltValue(arms string, depth int) (lValue, error) {
	parts := strings.Split(arms, ".")
	if len(parts) != 2 {
		return lValue{}, refuse("Count$Revolt.%s", arms)
	}
	var br [2]lValue
	for i, t := range parts {
		var err error
		if br[i], err = lc.headArm("Revolt", t, depth); err != nil {
			return lValue{}, err
		}
	}
	return lValue{Kind: vCompare, N: 1, Op: opGE, Sub: []lValue{{Kind: vRevolt}, {Kind: vBranches, Sub: br[:]}}}, nil
}
