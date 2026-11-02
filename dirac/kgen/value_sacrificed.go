package kgen

import "fmt"

// sacrificedThisTurnValue lowers PlayerCountPropertyYou$SacrificedThisTurn
// <spec> (gorge engine/effects/count_cov3.go sacrificedThisTurn;
// ) to ExSacrificedThisTurn. The kernel keeps three tallies
// per player, so only the three exact spellings read: Permanent (N 0,
// every sacrifice), Artifact (N 1, artifact cards) and Food (N 2, the printed
// Food subtype). Any other spec, including the empty one, refuses by name;
// an /Op suffix on the head applies to the count (svarValue).
func sacrificedThisTurnValue(rest string) (lValue, error) {
	switch rest {
	case "SacrificedThisTurn Permanent":
		return lValue{Kind: vSacrificedThisTurn, N: 0}, nil
	case "SacrificedThisTurn Artifact":
		return lValue{Kind: vSacrificedThisTurn, N: 1}, nil
	case "SacrificedThisTurn Food":
		return lValue{Kind: vSacrificedThisTurn, N: 2}, nil // the Food cards
	}
	return lValue{}, fmt.Errorf("PlayerCountPropertyYou$%s", rest)
}
