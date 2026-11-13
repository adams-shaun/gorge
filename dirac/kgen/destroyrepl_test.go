package kgen

import "testing"

// TestDestroyReplacementRefused: an R:Event$ Destroy carrier whose line is not a
// lowered shape refuses by name. Mossbridge Troll's self-regenerate line lowers
// (lower_destroyregen.go); Clergy and Knight of the Holy Nimbus carry the same
// line and refuse only for their opponent-activated CantRegenerate Effect.
// Pyramids carries its line on an Effect.
func TestDestroyReplacementRefused(t *testing.T) {
	const why = "repl-hard: Destroy replacement"
	const noRegen = "Effect static mode CantRegenerate"
	checkCarrier(t, []string{"Mossbridge Troll"}, map[string]string{
		"Knight of the Holy Nimbus": noRegen,
		"Clergy of the Holy Nimbus": noRegen,
		"Crackling Emergence":       why,
		"Harmonious Emergence":      why,
		"Pyramids":                  "",
	}, nil)
}
