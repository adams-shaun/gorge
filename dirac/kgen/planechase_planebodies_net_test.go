package kgen

import (
	"strings"
	"testing"
)

// planebodiesNetPlane builds a synthetic plane script around one body line
// (a T: chaos dig or an R: replacement plus its SVars).
func planebodiesNetPlane(body string) string {
	return "Name:Planebodies Net Plane\nManaCost:no cost\nTypes:Plane Probe\n" + body + "Oracle:probe\n"
}

// TestPlanechasePlaneBodiesNetRefusesScopedShapes pins the cs2 tightenings:
// the shapes where gorge and the kernel's plane model read differently refuse
// by name, while the exact lowered shape stays accepted (the precondition each
// refusal assertion depends on).
//   - a planar dig with FoundDestination$ absent (gorge joins the found card
//     into the revealed pile in reveal order; the kernel moves found first);
//   - a plane CreateToken line with Amount$ Thrice (the kernel's plane count
//     is a flat x2) or a ValidCard$ scope (the kernel's count is unscoped).
func TestPlanechasePlaneBodiesNetRefusesScopedShapes(t *testing.T) {
	const dig = "T:Mode$ ChaosEnsues | TriggerZones$ Command | Execute$ NetDig | TriggerDescription$ probe\n" +
		"SVar:NetDig:DB$ DigUntil | Valid$ Plane | DigZone$ PlanarDeck | RememberFound$ True | %sRevealedDestination$ PlanarDeck | FoundLibraryPosition$ -1 | RevealedLibraryPosition$ -1\n"
	withFound := strings.Replace(dig, "%s", "FoundDestination$ PlanarDeck | ", 1)
	noFound := strings.Replace(dig, "%s", "", 1)
	tok := func(rline, amount string) string {
		return "R:Event$ CreateToken | ActiveZones$ Command | ReplaceWith$ NetTok | EffectOnly$ True" + rline + " | Description$ probe\n" +
			"SVar:NetTok:DB$ ReplaceToken | Type$ Amount" + amount + "\n"
	}
	cases := []struct {
		name, src, want string // want == "" means the shape must lower clean of the net's refusals
	}{
		{"dig with FoundDestination lowers", withFound, ""},
		{"dig without FoundDestination", noFound, "DigUntil planar scan without FoundDestination$ PlanarDeck"},
		{"token x2 lowers", tok("", ""), ""},
		{"token Twice lowers", tok("", " | Amount$ Twice"), ""},
		{"token Thrice", tok("", " | Amount$ Thrice"), "CreateToken Amount \"Thrice\" on a plane face"},
		{"token ValidCard", tok(" | ValidCard$ Card", ""), "CreateToken ValidCard on a plane face"},
	}
	for _, c := range cases {
		got := lowerPlaneProbe(t, planebodiesNetPlane(c.src))
		if c.want == "" {
			for _, bad := range []string{"without FoundDestination", "on a plane face", "CreateToken Amount"} {
				if strings.Contains(got, bad) {
					t.Errorf("%s: unexpected refusal %q in %q", c.name, bad, got)
				}
			}
			continue
		}
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: want refusal %q, got %q", c.name, c.want, got)
		}
	}
}
