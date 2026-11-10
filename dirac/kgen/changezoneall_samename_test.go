package kgen

import (
	"strings"
	"testing"
)

// TestChangeZoneAllSameNameLowers: Rat King, Verminister's return (its
// graveyard target and every other card in your graveyard with its name)
// lowers to one ChangeZoneAll whose Card.YouOwn clause reads
// PredSameNameT0. Legions to Ashes' battlefield token sweep lowers to two
// OR'd clauses (the target itself, then OppCtrl tokens reading
// PredSameNameT0) exiling; The End's TargetedCard.sameName search refuses,
// naming its spelling.
func TestChangeZoneAllSameNameLowers(t *testing.T) {
	refused := map[string]string{
		"The End": `"TargetedCard.sameName"`,
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
		if c.Name == "Legions to Ashes" {
			checkLegionsToAshes(t, c)
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

// checkLegionsToAshes: the sweep is one battlefield-to-exile ChangeZoneAll
// whose filter is the target (targetedBy) OR an opponent-controlled token
// reading PredSameNameT0.
func checkLegionsToAshes(t *testing.T, c *lCard) {
	t.Helper()
	for _, r := range c.Refusals {
		t.Errorf("%s: refused: %s", c.Name, r.Reason)
	}
	found := false
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if in.Op != "ChangeZoneAll" || in.F == nil {
				continue
			}
			found = true
			cl := in.F.Clauses
			if len(cl) != 2 {
				t.Errorf("%s: sweep filter %+v, want 2 clauses", c.Name, cl)
				continue
			}
			if cl[0].Flags&fTargetedBy == 0 {
				t.Errorf("%s: clause 0 %+v, want targetedBy", c.Name, cl[0])
			}
			if cl[1].Flags&fToken == 0 || cl[1].Ctrl != whoOpp || cl[1].CmpStat != cmpSameNameT0 {
				t.Errorf("%s: clause 1 %+v, want a token OppCtrl clause reading PredSameNameT0", c.Name, cl[1])
			}
			if in.C != zExile {
				t.Errorf("%s: sweep goes to zone %d, want exile", c.Name, in.C)
			}
		}
	}
	if !found {
		t.Errorf("%s: no ChangeZoneAll lowered", c.Name)
	}
}
