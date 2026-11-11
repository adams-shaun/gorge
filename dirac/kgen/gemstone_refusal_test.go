package kgen

import (
	"strings"
	"testing"
)

// TestGemstoneCavernsRefusedByName: Gemstone Caverns stays refused on its
// opening-hand node, with the reason naming what the kernel lacks
// (lower_gemstone.go): the !PlayFirst-gated effect. Its Condition-split mana
// production lowers (lower_condsplit.go), River of Tears' with it; a plain
// Leyline still lowers (the FromHand case is untouched).
func TestGemstoneCavernsRefusedByName(t *testing.T) {
	want := map[string][]string{
		"Gemstone Caverns": {"not being the starting player (!PlayFirst"},
		"River of Tears":   {},
	}
	seen := map[string]bool{}
	for _, c := range lowerNamed(t, []string{"Gemstone Caverns", "River of Tears", "Leyline of the Void"}) {
		seen[c.Name] = true
		var all []string
		for _, r := range c.Refusals {
			all = append(all, r.Reason)
		}
		got := strings.Join(all, "; ")
		if c.Name == "Leyline of the Void" {
			if len(c.Refusals) != 0 || !c.Leyline {
				t.Errorf("Leyline of the Void: refusals %q, Leyline %v; want accepted as a Leyline", got, c.Leyline)
			}
			continue
		}
		if len(c.Refusals) != len(want[c.Name]) {
			t.Errorf("%s: %d refusals %q, want %d", c.Name, len(c.Refusals), got, len(want[c.Name]))
		}
		for _, w := range want[c.Name] {
			if !strings.Contains(got, w) {
				t.Errorf("%s: refusals %q, want one containing %q", c.Name, got, w)
			}
		}
	}
	for _, n := range []string{"Gemstone Caverns", "River of Tears", "Leyline of the Void"} {
		if !seen[n] {
			t.Errorf("precondition: %s not lowered (corpus missing?)", n)
		}
	}
}
