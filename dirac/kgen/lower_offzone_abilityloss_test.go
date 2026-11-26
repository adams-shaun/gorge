package kgen

import (
	"strings"
	"testing"
)

// TestStaticZonesOffZoneAbilityLoss: Yixlid Jailer's graveyard removal lowers
// to exactly one GraveNoAbilities static (kop.StatGraveNoAbilities) over
// Card in the graveyard, and the card carries no refusal (the kernel's
// graveyard readers are all wired, dsub-jailer).
func TestStaticZonesOffZoneAbilityLoss(t *testing.T) {
	c := lowerNamedCard(t, "Yixlid Jailer")
	n := 0
	for _, st := range c.Statics {
		if st.Kind == "GraveNoAbilities" {
			n++
			if st.Zone != int(zGraveyard) || st.F == nil {
				t.Errorf("static zone %d filter %v, want the graveyard and a Card filter", st.Zone, st.F)
			}
		}
	}
	if n != 1 || len(c.Statics) != 1 {
		t.Errorf("Yixlid Jailer lowers to %d GraveNoAbilities statics of %d, want exactly one", n, len(c.Statics))
	}
	if len(c.Refusals) != 0 {
		t.Errorf("Yixlid Jailer refusals = %v, want none", c.Refusals)
	}
}

// TestStaticZonesOffZoneAbilityLossShapes: every shape beside the corpus one
// refuses by name.
func TestStaticZonesOffZoneAbilityLossShapes(t *testing.T) {
	for _, tc := range []struct{ name, static, want string }{
		{"Loss Probe Creature", "Mode$ Continuous | Affected$ Creature | AffectedZone$ Graveyard | RemoveAllAbilities$ True", `Affected "Creature"`},
		{"Loss Probe Extra", "Mode$ Continuous | Affected$ Card | AffectedZone$ Graveyard | RemoveAllAbilities$ True | AddKeyword$ Flying", "beside AddKeyword$"},
	} {
		c := lowerScriptCard(t, tc.name, "Name:"+tc.name+"\nManaCost:1 B\nTypes:Creature Zombie\nPT:2/1\nS:"+tc.static+"\nOracle:probe\n")
		hit := false
		for _, rf := range c.Refusals {
			hit = hit || strings.Contains(rf.Reason, tc.want)
		}
		if !hit {
			t.Errorf("%s refusals = %v, want one containing %q", tc.name, c.Refusals, tc.want)
		}
	}
}
