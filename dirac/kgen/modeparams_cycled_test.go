package kgen

import (
	"strings"
	"testing"
)

// TestModeParamCycledOtherLowers: "whenever you cycle another card" (a
// battlefield Cycled line whose ValidCard$ names Card.Other) lowers like the
// bare "a card" line: the cycled card is the one in the graveyard, never the
// listener on the battlefield, so Other restricts nothing (gorge cycledMatches
// reads ValidCard$ against the moved card).
func TestModeParamCycledOtherLowers(t *testing.T) {
	names := []string{"Astral Drift", "Drannith Healer", "Drannith Stinger", "Flourishing Fox"}
	for _, c := range lowerNamed(t, names) {
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s", c.Name, r.Reason)
		}
		cyc := 0
		for _, tr := range trigsOf(c, "Discarded") {
			if tr.Cycled {
				cyc++
			}
		}
		// Astral Drift also carries its own "when you cycle this card" line
		if want := map[string]int{"Astral Drift": 2}[c.Name]; cyc != max(want, 1) {
			t.Errorf("%s: %d cycled lines lowered, want %d", c.Name, cyc, max(want, 1))
		}
	}
}

// TestModeParamCycledOtherRefuses: the Cycled shapes the kernel still does not
// read refuse by name, a FirstTime$ rider and an off-battlefield zone.
func TestModeParamCycledOtherRefuses(t *testing.T) {
	want := map[string]string{"Valiant Rescuer": "FirstTime", "Benalish Partisan": "Cycled ValidCard"}
	for _, c := range lowerNamed(t, []string{"Valiant Rescuer", "Benalish Partisan"}) {
		if len(c.Refusals) == 0 || !strings.Contains(c.Refusals[0].Reason, want[c.Name]) {
			t.Errorf("%s: refusals %v, want one naming %q", c.Name, c.Refusals, want[c.Name])
		}
	}
}
