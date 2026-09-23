package effects

import "testing"

// TestSpecNeedsResolver pins the shape census SpecNeedsResolver reports: a
// numeric predicate whose right-hand side is an SVar name rather than a
// literal is exactly what MatchesSpec/MatchesSpecFrom's noResolve path can
// only ever answer "no" to. Callers with no resolver (rules' ETB-copy
// whitelist) withhold instead of offering an empty list.
func TestSpecNeedsResolver(t *testing.T) {
	for _, tc := range []struct {
		spec string
		want bool
	}{
		{"Creature.Other+cmcLEY", true}, // Mockingbird
		{"Creature.YouCtrl+powerGEX", true},
		{"Creature.counters_GEY_P1P1", true},
		{"Creature.Other", false},
		{"Creature.YouCtrl+powerGE4", false}, // Deceptive Frostkite
		{"Creature.counters_EQ0_P1P1", false},
		{"Creature.powerLTtoughness", false},
		{"Artifact.Other,Creature.Other+cmcLEY", true},
		{"Permanent.nonLand+Other", false},
		{"", false},
	} {
		if got := SpecNeedsResolver(tc.spec); got != tc.want {
			t.Errorf("SpecNeedsResolver(%q) = %v, want %v", tc.spec, got, tc.want)
		}
	}
}
