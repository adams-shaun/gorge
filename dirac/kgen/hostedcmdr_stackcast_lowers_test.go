package kgen

import (
	"strings"
	"testing"
)

// TestHostedCmdrStackCastLowers pins the three vivi-ornitier-cedh stack and
// cast blockers lowered decision-for-decision (dirac gap cli-20261127T171908Z):
//
//   - Commandeer: api ControlSpell (the kernel's gain-control op);
//   - Disrupting Shoal: DB$ Counter | Defined$ Targeted cast as a spell, the
//     target read from the cast's own target list (no SpellCast trigger);
//   - Blazing Shoal: the Announce$ X static read for the spell's X.
//
// Each lands unverified (locked down by the dirac/oracle lockstep tests); the
// kgen refusals they used to raise are gone.
func TestHostedCmdrStackCastLowers(t *testing.T) {
	cs := lowerNamed(t, []string{"Commandeer", "Disrupting Shoal", "Blazing Shoal"})
	seen := map[string]bool{}
	for _, c := range cs {
		if c.Name != "Commandeer" && c.Name != "Disrupting Shoal" && c.Name != "Blazing Shoal" {
			continue
		}
		seen[c.Name] = true
		var why []string
		for _, r := range c.Refusals {
			why = append(why, r.Reason)
		}
		if got := strings.Join(why, "; "); got != "" {
			t.Errorf("%s refused: %s", c.Name, got)
		}
	}
	if len(seen) != 3 {
		t.Fatalf("saw %d of 3 cards in the lowered set", len(seen))
	}
}
