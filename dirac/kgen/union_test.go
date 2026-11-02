package kgen

import (
	"strings"
	"testing"
)

// kunionCarriers: Defined$ Targeted over a SubAbility$ chain of several
// target slots lowers to the chain's target union (gorge chainTargetUnion),
// not the last slot.
var kunionCarriers = map[string]string{
	"Fumarole":          "Destroy obj=Targets01",
	"Plague Spores":     "Destroy obj=Targets01 C=1",
	"Spiteful Blow":     "Destroy obj=Targets01",
	"Stomp and Howl":    "Destroy obj=Targets01",
	"Decimate":          "Destroy obj=Targets0123",
	"Boom Box":          "Destroy obj=Targets012",
	"Goblin Grenadiers": "Destroy obj=Targets01",
	"Arm the Cathars":   "GrantKeyword obj=Targets012 CS=Vigilance",
	"Relive the Past":   "Return obj=Targets012 C=1",
	"Hull Breach":       "Destroy obj=Targets23",
	"Reign of Chaos":    "Destroy obj=Targets01",
	"Boom":              "Destroy obj=Targets01",
}

func TestUnionCarriers(t *testing.T) {
	var names []string
	for n := range kunionCarriers {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		want, ok := kunionCarriers[c.Name]
		if !ok {
			continue
		}
		seen++
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s", c.Name, r.Reason)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), want) {
			t.Errorf("%s: lowered IR lacks %q:\n%s", c.Name, want, sb.String())
		}
	}
	if seen != len(kunionCarriers) {
		t.Errorf("lowered %d of %d carriers", seen, len(kunionCarriers))
	}
}
