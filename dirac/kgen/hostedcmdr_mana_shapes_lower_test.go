package kgen

import (
	"strings"
	"testing"
)

// TestHostedCmdrManaShapesLower pins the three hosted-Commander mana shapes
// this ticket started from. Virtue of Strength lowers: its ProduceMana
// replacement over basic lands becomes a reflected tap-extra static (the
// same shape as Mana Flare's "one more of the produced type", in the
// unpriced StTapReflectDefer mode so the plan matches gorge's). Squandered
// Resources (ManaReflected over the sacrificed land) and Vivi Ornitier (a
// PlayerTurn$ gate on a {0} dynamic Combo mana ability) still have no kernel
// mechanism: each stays refused with its own named reason, so a future
// lowering cannot silently change what the table holds.
func TestHostedCmdrManaShapesLower(t *testing.T) {
	cs := lowerNamed(t, []string{"Virtue of Strength", "Squandered Resources", "Vivi Ornitier"})
	want := map[string]string{
		"Virtue of Strength":   "",
		"Squandered Resources": `ManaReflected Valid$ "Defined.Sacrificed"`,
		"Vivi Ornitier":        `activation PlayerTurn$ "True" beside another condition`,
	}
	seen := 0
	for _, c := range cs {
		w, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		var why []string
		for _, r := range c.Refusals {
			why = append(why, r.Reason)
		}
		got := strings.Join(why, "; ")
		if w == "" {
			if got != "" {
				t.Errorf("%s refused: %s", c.Name, got)
				continue
			}
			found := false
			for _, st := range c.Statics {
				if st.Kind != "TapExtra" {
					continue
				}
				for _, fl := range st.Flags {
					if fl == "TapReflectDefer" && st.TapAmt == 2 {
						found = true
					}
				}
			}
			if !found {
				t.Errorf("%s has no reflected tap-extra static with amount 2: %+v", c.Name, c.Statics)
			}
			continue
		}
		if !strings.Contains(got, w) {
			t.Errorf("%s refusal = %q, want it to contain %q", c.Name, got, w)
		}
	}
	if seen != len(want) {
		t.Fatalf("lowered %d of %d cards", seen, len(want))
	}
}
