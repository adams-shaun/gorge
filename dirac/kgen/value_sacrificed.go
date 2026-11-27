package kgen

import (
	"fmt"
	"strings"
)

// sacrificedThisTurnValue lowers PlayerCountPropertyYou$SacrificedThisTurn
// <spec> (gorge engine/effects/count_cov3.go sacrificedThisTurn) to
// ExSacrificedThisTurn. The three spellings the kernel keeps as per-player
// tallies stay tallies: Permanent (N 0, every sacrifice), Artifact (N 1,
// artifact cards) and Food (N 2, the printed Food subtype). Every other spec
// is a filter read over the turn's sacrifices, matched against the card
// where it went (X.F; the kernel's sacrificedThisTurn and matchDeparture), so
// Dargo's Artifact,Creature and Evendo's Permanent.!token widen the value
// without a State field. An /Op suffix on the head applies to the count
// (svarValue).
func (lc *lowerCtx) sacrificedThisTurnValue(rest string) (lValue, error) {
	switch rest {
	case "SacrificedThisTurn Permanent":
		return lValue{Kind: vSacrificedThisTurn, N: 0}, nil
	case "SacrificedThisTurn Artifact":
		return lValue{Kind: vSacrificedThisTurn, N: 1}, nil
	case "SacrificedThisTurn Food":
		return lValue{Kind: vSacrificedThisTurn, N: 2}, nil // the Food cards
	}
	spec, ok := strings.CutPrefix(rest, "SacrificedThisTurn ")
	if !ok || strings.TrimSpace(spec) == "" {
		return lValue{}, fmt.Errorf("PlayerCountPropertyYou$%s", rest)
	}
	f, err := lc.filter(spec)
	if err != nil {
		return lValue{}, err
	}
	return lValue{Kind: vSacrificedThisTurn, F: f}, nil
}
