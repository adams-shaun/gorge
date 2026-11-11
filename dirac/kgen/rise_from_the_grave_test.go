package kgen

import (
	"strings"
	"testing"
)

// TestRiseFromTheGraveAddColor pins a ChangeZone StaticEffect$ AddColor$:
// Rise from the Grave's "black Zombie in addition to its other colors and
// types" lowers to one override record adding the Zombie subtype and black
// (gorge parseStaticEffectGrant AddColor$, layer 5, CR 613.1e).
func TestRiseFromTheGraveAddColor(t *testing.T) {
	cs := lowerNamed(t, []string{"Rise from the Grave"})
	seen := 0
	for _, c := range cs {
		var why []string
		for _, r := range c.Refusals {
			why = append(why, r.Reason)
		}
		got := strings.Join(why, "; ")
		var sb strings.Builder
		dumpLIR(&sb, c)
		lir := sb.String()
		if c.Name != "Rise from the Grave" {
			continue
		}
		seen++
		if got != "" {
			t.Errorf("Rise from the Grave refused: %s", got)
		}
		if !strings.Contains(lir, "Override obj=Target0") {
			t.Errorf("Rise from the Grave LIR lacks the returned object's override:\n%s", lir)
		}
		if len(c.overrides) != 1 {
			t.Fatalf("Rise from the Grave: %d override records, want 1", len(c.overrides))
		}
		o := c.overrides[0]
		if o.AddColors != "B" || len(o.AddSubtypes) != 1 || o.AddSubtypes[0] != "Zombie" || o.HasSetColors {
			t.Errorf("Rise from the Grave override: AddColors %q AddSubtypes %v HasSetColors %v, want B, [Zombie], false", o.AddColors, o.AddSubtypes, o.HasSetColors)
		}
	}
	if seen != 1 {
		t.Fatalf("lowered %d of Rise from the Grave", seen)
	}
}
