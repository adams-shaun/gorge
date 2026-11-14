package kgen

import (
	"strings"
	"testing"
)

// TestStaticProtYourColors: Empty-Shrine Kannushi's self grant of "Protection
// from colorsYouCtrl" lowers as its own static kind (the quality is the
// recipient's controller's colours, not a filter over the source), and any
// shape beside that exact AddKeyword$/AffectedDefined$ pair refuses by name.
func TestStaticProtYourColors(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Empty-Shrine Kannushi", form: "static AddKeyword$ Protection from colorsYouCtrl", lir: "static ProtYourColors"},
	})
	for _, c := range []struct{ name, static, want string }{
		{"another Affected$ shape refuses", "S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddKeyword$ Protection from colorsYouCtrl | Description$ x", "Protection from colorsYouCtrl Affected"},
		{"a condition refuses", "S:Mode$ Continuous | AffectedDefined$ Self | AddKeyword$ Protection from colorsYouCtrl | Condition$ Threshold | Description$ x", "Protection from colorsYouCtrl static with a condition"},
		{"another keyword beside it refuses", "S:Mode$ Continuous | AffectedDefined$ Self | AddKeyword$ Protection from colorsYouCtrl & Flying | Description$ x", `keyword "Protection from colorsYouCtrl"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := kcantselfRefusals(t, c.static)
			if !strings.Contains(got, c.want) {
				t.Errorf("want refusal %q, got %q", c.want, got)
			}
		})
	}
}
