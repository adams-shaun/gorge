package kgen

import (
	"strings"
	"testing"
)

// TestStaticHexproofOwnColors: Tam, Mindful First-Year's granted
// "hexproof from each of its colours" lowers as its own static kind (no
// keyword bit is free in kop.KwMask), and any shape beside that exact
// AddKeyword$/Affected$ pair refuses by name.
func TestStaticHexproofOwnColors(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Tam, Mindful First-Year", form: "static AddKeyword$ Hexproof:CardColors", lir: "static HexproofOwnColors"},
	})
	for _, c := range []struct{ name, static, want string }{
		{"another Affected$ shape refuses", "S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddKeyword$ Hexproof:CardColors | Description$ x", "Hexproof:CardColors Affected"},
		{"a condition refuses", "S:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Hexproof:CardColors | Condition$ Threshold | Description$ x", "Hexproof:CardColors static with a condition"},
		{"another keyword beside it refuses", "S:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Hexproof:CardColors & Flying | Description$ x", `keyword "Hexproof:CardColors"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := kcantselfRefusals(t, c.static)
			if !strings.Contains(got, c.want) {
				t.Errorf("want refusal %q, got %q", c.want, got)
			}
		})
	}
}
