package kgen

import (
	"strings"
	"testing"
)

// TestChangeZoneAllSameNameLowers: Rat King, Verminister's return (its
// graveyard target and every other card in your graveyard with its name)
// lowers to one ChangeZoneAll whose Card.YouOwn clause reads
// PredSameNameT0. Legions to Ashes' battlefield token form (ControlledBy
// TargetedController, which the kernel's filters do not read) and The End's
// TargetedCard.sameName search refuse, each naming its spelling.
func TestChangeZoneAllSameNameLowers(t *testing.T) {
	refused := map[string]string{
		"Legions to Ashes": "same-name tokens ControlledBy TargetedController",
		"The End":          `"TargetedCard.sameName"`,
	}
	seen := map[string]bool{}
	for _, c := range lowerNamed(t, []string{"Rat King, Verminister", "Legions to Ashes", "The End"}) {
		seen[c.Name] = true
		if want, ok := refused[c.Name]; ok {
			hit := false
			for _, r := range c.Refusals {
				hit = hit || strings.Contains(r.Reason, want)
			}
			if !hit {
				t.Errorf("%s: refusals %v, want one naming %s", c.Name, c.Refusals, want)
			}
			continue
		}
		if c.Name != "Rat King, Verminister" {
			continue
		}
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s", c.Name, r.Reason)
		}
		found := false
		for _, a := range c.Abilities {
			for _, in := range a.Prog {
				if in.Op != "ChangeZoneAll" || in.F == nil {
					continue
				}
				if len(a.Targets) != 1 || a.Targets[0].Zone != zGraveyard {
					t.Errorf("%s: sweep beside targets %+v, want one graveyard target", c.Name, a.Targets)
				}
				if len(in.F.Clauses) != 1 || in.F.Clauses[0].CmpStat != cmpSameNameT0 || in.F.Clauses[0].Owner != whoYou {
					t.Errorf("%s: sweep filter %+v, want one Card.YouOwn clause reading PredSameNameT0", c.Name, in.F.Clauses)
				}
				if !strings.Contains(in.CS, "from=Graveyard") || !strings.Contains(in.CS, "|Tapped") || in.C != zBattlefield {
					t.Errorf("%s: sweep CS %q C %d, want graveyard to battlefield tapped", c.Name, in.CS, in.C)
				}
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no ChangeZoneAll lowered", c.Name)
		}
	}
	for _, n := range []string{"Rat King, Verminister", "Legions to Ashes", "The End"} {
		if !seen[n] {
			t.Errorf("%s not lowered at all (corpus missing it?)", n)
		}
	}
}
