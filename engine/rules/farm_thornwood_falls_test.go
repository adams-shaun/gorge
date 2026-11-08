package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm window class 3way:FX|G (DFT-s503/s507/s640/s456/s458, field
// `permanents`, Thornwood Falls on the board): Forge and XMage print a
// Vehicle that became a creature under March of the World Ooze as
// "artifact creature vehicle 6/6"; gorge prints "artifact creature ooze
// vehicle 6/6". Both set 6/6, so the Vehicle is a creature when layer 7b
// runs; they disagree only on layer 4, where they order March's type grant
// by timestamp and so run it before the animation makes the Vehicle a
// creature. CR 613.8a: March's grant depends on the animation (applying it
// changes what the grant applies to), so the animation applies first, in the
// same layer, whatever the timestamps, and the Vehicle is an Ooze. gorge is
// right; Thornwood Falls is only a bystander on those boards. The crew case
// is pinned by TestCrewedVehicleIsOozeUnderMarchOfTheWorldOoze; this pins
// the exhaust animation (Rangers' Refueler, a permanent "becomes an artifact
// creature") the farm windows hit, with the land in play.
func TestFarmThornwoodFalls(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	scenario := `{
	  "name": "farm-thornwood-falls-exhaust-ooze",
	  "setup": {"p0": {"battlefield": ["Rangers' Refueler", "March of the World Ooze", "Thornwood Falls", "Island", "Island", "Island", "Island"]}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Rangers' Refueler", "mana": "UUUU", "ability_index": 0},
	    {"op": "pass", "seat": 0},
	    {"op": "pass", "seat": 1},
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
	last := res.Snapshots[len(res.Snapshots)-1]
	var sawLand, sawVehicle bool
	for _, p := range last.Permanents {
		switch p.Ref {
		case "p0:Thornwood Falls":
			sawLand = true
		case "p0:Rangers' Refueler":
			sawVehicle = true
			types := map[string]bool{}
			for _, ty := range p.Types {
				types[ty] = true
			}
			// Precondition: the animation resolved, so the Vehicle is a 6/6
			// creature with base 6/6 plus the exhaust +1/+1 counter, the P/T half both
			// oracles agree on.
			if !types["Creature"] || p.PT != "7/7" {
				t.Fatalf("exhausted Refueler is %v %s, want a 7/7 creature (base 6/6 plus the exhaust counter)", p.Types, p.PT)
			}
			if !types["Ooze"] || !types["Vehicle"] || !types["Artifact"] {
				t.Fatalf("exhausted Refueler is %v, want an Ooze Vehicle artifact creature (CR 613.8a)", p.Types)
			}
		}
	}
	if !sawLand || !sawVehicle {
		t.Fatalf("board lost a permanent (land %v, vehicle %v): %+v", sawLand, sawVehicle, last.Permanents)
	}
}
