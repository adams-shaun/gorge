package kgen

import (
	"strings"
	"testing"
)

// TestExileToGraveyardScope: the targeted exile->graveyard move and the
// duration-exile link lift cover the two activated abilities that read
// ExiledWithSource at target time (Bronzebeak Foragers, Gelatinous Cube) and
// nothing else. Pull from Eternity (any face-up exiled card) and Mysterious
// Limousine (a resolution-time ChangeZoneAll reader of a duration exile) have
// no test of their own, so they stay refused.
func TestExileToGraveyardScope(t *testing.T) {
	want := map[string]string{ // "" lowers; otherwise the refusal text
		"Bronzebeak Foragers":  "",
		"Gelatinous Cube":      "",
		"Pull from Eternity":   "ChangeZone to graveyard",
		"Mysterious Limousine": "ExileUntilLeaves",
	}
	var names []string
	for n := range want {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		w, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		var rs []string
		for _, r := range c.Refusals {
			rs = append(rs, r.Reason)
		}
		got := strings.Join(rs, "; ")
		if w == "" && got != "" {
			t.Errorf("%s: refused: %s", c.Name, got)
		}
		if w != "" && !strings.Contains(got, w) {
			t.Errorf("%s: refusals %q, want one naming %q", c.Name, got, w)
		}
	}
	if seen != len(want) {
		t.Errorf("lowered %d of %d cards", seen, len(want))
	}
}
