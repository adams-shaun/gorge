package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGemstoneCavernsLowersAsLeylineGem: Gemstone Caverns' !PlayFirst-gated
// opening-hand effect lowers onto the Leyline ask with the Gemstone marks
// (lower_gemstone.go: one luck counter, then the hand exile); its
// Condition-split mana production lowers (lower_condsplit.go), River of
// Tears' with it; a plain Leyline lowers without the marks; Impatient
// Iguana's gated RevealCard shape stays refused, naming the gate.
func TestGemstoneCavernsLowersAsLeylineGem(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range lowerNamed(t, []string{"Gemstone Caverns", "River of Tears", "Leyline of the Void", "Impatient Iguana"}) {
		seen[c.Name] = true
		var all []string
		for _, r := range c.Refusals {
			all = append(all, r.Reason)
		}
		got := strings.Join(all, "; ")
		switch c.Name {
		case "Gemstone Caverns":
			if len(c.Refusals) != 0 || !c.Leyline || !c.LeylineGem || c.LeylineCtr != kop.CtrLuck || c.LeylineCtrN != 1 {
				t.Errorf("Gemstone Caverns: refusals %q, Leyline %v gem %v ctr %d/%d; want a Leyline with one luck counter",
					got, c.Leyline, c.LeylineGem, c.LeylineCtr, c.LeylineCtrN)
			}
		case "River of Tears":
			if len(c.Refusals) != 0 {
				t.Errorf("River of Tears: refusals %q", got)
			}
		case "Leyline of the Void":
			if len(c.Refusals) != 0 || !c.Leyline || c.LeylineGem {
				t.Errorf("Leyline of the Void: refusals %q, Leyline %v gem %v; want a plain Leyline", got, c.Leyline, c.LeylineGem)
			}
		case "Impatient Iguana":
			if !strings.Contains(got, "not being the starting player (RevealCard:!PlayFirst)") {
				t.Errorf("Impatient Iguana: refusals %q, want the gated RevealCard refusal", got)
			}
		}
	}
	for _, n := range []string{"Gemstone Caverns", "River of Tears", "Leyline of the Void", "Impatient Iguana"} {
		if !seen[n] {
			t.Errorf("precondition: %s not lowered (corpus missing?)", n)
		}
	}
}
