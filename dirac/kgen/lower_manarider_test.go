package kgen

import (
	"strings"
	"testing"
)

// TestManaRiderProgram: a plain {T} any-colour mana ability whose rider puts
// a counter on its own source (Twitching Doll) lowers with the rider as its
// program; a rider of another shape (Chromatic Sphere's draw, Eumidian
// Hatchery's counter beside a life cost) is still refused.
func TestManaRiderProgram(t *testing.T) {
	want := map[string]bool{"Twitching Doll": true, "Chromatic Sphere": false, "Eumidian Hatchery": false}
	var names []string
	for n := range want {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		lowers, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		refused := false
		for _, r := range c.Refusals {
			refused = refused || strings.Contains(r.Reason, "mana ability with a rider program")
		}
		if lowers {
			if len(c.Refusals) != 0 {
				t.Errorf("%s refused: %v", c.Name, c.Refusals)
			}
			var sb strings.Builder
			dumpLIR(&sb, c)
			if !strings.Contains(sb.String(), "PutCounter") || !strings.Contains(sb.String(), "NEST") {
				t.Errorf("%s: the mana ability's rider is missing:\n%s", c.Name, sb.String())
			}
			found := false
			for _, a := range c.Abilities {
				found = found || ctrRiderMana(a)
			}
			if !found {
				t.Errorf("%s: no ability is the counter-rider mana shape", c.Name)
			}
		} else if !refused {
			t.Errorf("%s: not refused as a mana ability with a rider (%v)", c.Name, c.Refusals)
		}
	}
	if seen != len(want) {
		t.Errorf("lowered %d of %d cards", seen, len(want))
	}
}
