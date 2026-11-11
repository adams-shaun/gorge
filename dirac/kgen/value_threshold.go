package kgen

import "strings"

// thresholdValue lowers Count$Threshold.<yes>.<no> (gorge count_body_heads.go
// evalCountBodyDottedThreshold, CR 702.24a): <yes> when the resolving
// controller has seven or more cards in their graveyard, else <no>; each arm a
// literal or an SVar (Cabal Ritual's Amount$ X, Count$Threshold.5.3). The
// graveyard read is the card count of Card.YouOwn, which skips a token waiting
// to cease (CR 111.7) as gorge's CardCount does.
func (lc *lowerCtx) thresholdValue(arms string, depth int) (lValue, error) {
	parts := strings.Split(arms, ".")
	if len(parts) != 2 {
		return lValue{}, refuse("Count$Threshold.%s", arms)
	}
	f, err := lc.filter("Card.YouOwn")
	if err != nil {
		return lValue{}, err
	}
	var br [2]lValue
	for i, t := range parts {
		if br[i], err = lc.headArm("Threshold", t, depth); err != nil {
			return lValue{}, err
		}
	}
	return lValue{Kind: vCompare, N: 7, Op: opGE, Sub: []lValue{{Kind: vCountGraveyard, F: f}, {Kind: vBranches, Sub: br[:]}}}, nil
}
