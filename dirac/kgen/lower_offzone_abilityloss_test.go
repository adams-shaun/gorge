package kgen

import (
	"strings"
	"testing"
)

// TestStaticZonesOffZoneAbilityLoss: Yixlid Jailer's graveyard removal lowers
// to exactly one GraveNoAbilities static (kop.StatGraveNoAbilities) over
// Card in the graveyard, and the card stays refused with the named reason
// until the kernel's graveyard readers are all wired (graveLossReadersDone).
// Swirl the Mists keeps its refusal, which names the colour-word ticket.
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
	if graveLossReadersDone {
		if len(c.Refusals) != 0 {
			t.Errorf("Yixlid Jailer refusals = %v, want none once the readers are done", c.Refusals)
		}
	} else {
		hit := false
		for _, rf := range c.Refusals {
			hit = hit || strings.Contains(rf.Reason, graveLossPending)
		}
		if !hit {
			t.Errorf("Yixlid Jailer refusals = %v, want %q", c.Refusals, graveLossPending)
		}
	}

	sw := lowerNamedCard(t, "Swirl the Mists")
	hit := false
	for _, rf := range sw.Refusals {
		hit = hit || strings.Contains(rf.Reason, "agent-20261125T214327Z-46805547")
	}
	if !hit {
		t.Errorf("Swirl the Mists refusals = %v, want one naming agent-20261125T214327Z-46805547", sw.Refusals)
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
