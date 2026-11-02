package kgen

import (
	"strings"
	"testing"
)

// TestPermcopyLifts: the CopySpellAbility carriers of a
// permanent spell and the self-copies of an instant or sorcery lower.
func TestPermcopyLifts(t *testing.T) {
	want := []string{"Sage of the Skies", "Drafna, Founder of Lat-Nam", "Jin-Gitaxias, Progress Tyrant",
		"Mendicant Core, Guidelight", "Chancellor of Tales", "Social Snub", "Lumaret's Favor", "Photon Blast Barrage"}
	// Beluna's Gatekeeper puts the Adventure subtype in the pool's vocabulary
	cs := lowerNamed(t, append(want[:len(want):len(want)], "Beluna's Gatekeeper"))
	if len(cs) != len(want)+1 {
		t.Fatalf("%d cards, want %d", len(cs), len(want)+1)
	}
	for _, c := range cs {
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s", c.Name, r.Reason)
		}
	}
}

// TestPermcopyRefusals: the unread rider params stay refused.
func TestPermcopyRefusals(t *testing.T) {
	for _, c := range lowerNamed(t, []string{"Jackal, Genius Geneticist", "Tawnos, the Toymaker"}) {
		if len(c.Refusals) == 0 {
			t.Errorf("%s lowered, want a refusal for its unread param", c.Name)
			continue
		}
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, "permanent spell") {
				t.Errorf("%s: refused as a permanent spell (%s)", c.Name, r.Reason)
			}
		}
		t.Logf("%s: %s", c.Name, c.Refusals[0].Reason)
	}
}
