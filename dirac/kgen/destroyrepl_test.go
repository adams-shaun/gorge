package kgen

import (
	"strings"
	"testing"
)

// TestDestroyReplacementRefused: an R:Event$ Destroy carrier whose line is not a
// lowered shape refuses by name. Mossbridge Troll's self-regenerate line lowers
// (lower_destroyregen.go); Clergy and Knight of the Holy Nimbus carry the same
// line, so they must still refuse (their opponent-activated CantRegenerate
// Effect and other lines) but never on the Destroy replacement. Which of their
// other refusals comes first moves as sibling lowerings land, so the pin is
// "refuses, and not by the Destroy replacement". Pyramids carries its line on
// an Effect.
func TestDestroyReplacementRefused(t *testing.T) {
	const why = "repl-hard: Destroy replacement"
	checkCarrier(t, []string{"Mossbridge Troll"}, map[string]string{
		"Crackling Emergence":  why,
		"Harmonious Emergence": why,
		"Pyramids":             "",
	}, nil)
	_, ref := lowerCarriers(t, []string{"Knight of the Holy Nimbus", "Clergy of the Holy Nimbus"})
	for _, n := range []string{"Knight of the Holy Nimbus", "Clergy of the Holy Nimbus"} {
		if len(ref[n]) == 0 {
			t.Errorf("%s: lowers, want a refusal (its CantRegenerate Effect is not lowered)", n)
		}
		for _, r := range ref[n] {
			if strings.Contains(r, why) {
				t.Errorf("%s: refused on the self-regenerate Destroy replacement: %s", n, r)
			}
		}
	}
}
