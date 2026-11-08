package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm DFT-s458 windows xe:RAND:0:4:w15 and xe:RAND:4:4:w16 (attributed to
// Scrap Compactor, which only sat in the window): the first differing value
// is a Vehicle crewed under March of the World Ooze. gorge prints it as an
// Ooze, Forge and XMage do not. CR 613.8a sides with gorge: March's layer-4
// "creatures you control are Oozes" depends on crew's layer-4 "becomes an
// artifact creature" (applying crew changes what March applies to), so crew
// applies first despite its later timestamp and the Vehicle is an Ooze.
func TestFarmScrapCompactorWindowCrewedVehicleIsOoze(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct{ vehicle, crew string }{
		{"Rover Blades", "Grizzly Bears"},
		{"Rangers' Refueler", "Grizzly Bears"},
	} {
		t.Run(tc.vehicle, func(t *testing.T) {
			t.Parallel()
			scenario := `{
			  "name": "farm-crewed-vehicle-ooze",
			  "setup": {"p0": {"battlefield": ["` + tc.vehicle + `", "` + tc.crew + `", "March of the World Ooze"]}},
			  "steps": [
			    {"op": "activate", "seat": 0, "card": "p0:` + tc.vehicle + `", "answers": [{"kind": "choose", "pick": ["p0:` + tc.crew + `"]}], "ability_index": 1},
			    {"op": "pass", "seat": 0},
			    {"op": "pass", "seat": 1}
			  ]
			}`
			res, err := RunOracleScenarioJSON(reg, []byte(scenario))
			if err != nil {
				t.Fatalf("scenario JSON: %v", err)
			}
			if len(res.Fails) != 0 {
				t.Fatalf("%v", res.Fails)
			}
			if len(res.Snapshots) < 2 {
				t.Fatalf("want a snapshot before and after crew resolves, got %d", len(res.Snapshots))
			}
			ref := "p0:" + tc.vehicle
			// Precondition: with crew still on the stack the Vehicle is
			// neither a creature nor an Ooze, so March alone does not type it.
			if ty, _, ok := farmPermTypes(res.Snapshots[0], ref); !ok || ty["Creature"] || ty["Ooze"] {
				t.Fatalf("before crew resolves %s is %v (found=%v), want a non-creature non-Ooze", tc.vehicle, ty, ok)
			}
			ty, pt, ok := farmPermTypes(res.Snapshots[len(res.Snapshots)-1], ref)
			if !ok {
				t.Fatalf("%s not on the battlefield", tc.vehicle)
			}
			if !ty["Creature"] || !ty["Vehicle"] || !ty["Ooze"] || pt != "6/6" {
				t.Fatalf("crewed %s is %v %s, want an Ooze Vehicle creature 6/6", tc.vehicle, ty, pt)
			}
		})
	}
}

func farmPermTypes(s OracleSnapshot, ref string) (map[string]bool, string, bool) {
	for _, p := range s.Permanents {
		if p.Ref != ref {
			continue
		}
		ty := map[string]bool{}
		for _, x := range p.Types {
			ty[x] = true
		}
		return ty, p.PT, true
	}
	return nil, "", false
}
