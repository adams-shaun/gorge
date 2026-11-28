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
// Resources lowers to the sacrifice-reflected production (ManaSacrificed, read
// off the victim at each activation). Vivi Ornitier lowers to a finite-mask
// Combo Any gap over {U}/{R}: no tap, an ActivationLimit$ 1 once-each-turn
// bit, and its PlayerTurn$ gate on Trig.Cond.
func TestHostedCmdrManaShapesLower(t *testing.T) {
	cs := lowerNamed(t, []string{"Virtue of Strength", "Squandered Resources", "Vivi Ornitier"})
	want := map[string]bool{
		"Virtue of Strength":   true,
		"Squandered Resources": true,
		"Vivi Ornitier":        true,
	}
	seen := 0
	for _, c := range cs {
		if !want[c.Name] {
			continue
		}
		seen++
		var why []string
		for _, r := range c.Refusals {
			why = append(why, r.Reason)
		}
		if got := strings.Join(why, "; "); got != "" {
			t.Errorf("%s refused: %s", c.Name, got)
			continue
		}
		switch c.Name {
		case "Virtue of Strength":
			found := false
			for _, st := range c.Statics {
				if st.Kind != "TapExtra" {
					continue
				}
				for _, fl := range st.Flags {
					if fl == "TapReflectDefer" && st.TapAmt == 2 {
						found = true
						// the filter pins the replacement's reach: Land.Basic only
						// (a Land or Permanent filter would let nonbasics gain it)
						if st.F == nil || len(st.F.Clauses) != 1 {
							t.Errorf("%s static filter = %+v, want one Land.Basic clause", c.Name, st.F)
						} else if cl := st.F.Clauses[0]; cl.Types&tLand == 0 || (cl.Types|cl.TypesAll)&tBasic == 0 && cl.Flags&fBasic == 0 {
							t.Errorf("%s static filter clause = %+v, want Land.Basic", c.Name, cl)
						}
					}
				}
			}
			if !found {
				t.Errorf("%s has no reflected tap-extra static with amount 2: %+v", c.Name, c.Statics)
			}
		case "Squandered Resources":
			found := false
			for _, ab := range c.Abilities {
				if ab.Mana == nil || ab.Mana.Special != "ReflectedSacrificed" || ab.Cost.Tap || ab.Cost.Sac == nil || ab.Cost.SacN != 1 {
					continue
				}
				found = true
			}
			if !found {
				t.Errorf("%s has no sacrifice-reflected mana ability: %+v", c.Name, c.Abilities)
			}
		case "Vivi Ornitier":
			found := false
			for _, ab := range c.Abilities {
				if ab.Mana == nil || ab.Mana.Colors != "UR" || !ab.Mana.Choice || ab.Mana.Amount.isConst() ||
					!ab.TurnOnce || ab.Limit != 0 || ab.Cost.Tap || ab.ActCond.Kind != cYourTurn {
					continue
				}
				found = true
			}
			if !found {
				t.Errorf("%s has no finite-mask once-each-turn mana ability: %+v", c.Name, c.Abilities)
			}
		}
	}
	if seen != len(want) {
		t.Fatalf("lowered %d of %d cards", seen, len(want))
	}
}
