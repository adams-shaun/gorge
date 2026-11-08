package rules

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm windows DFT-s248, DFT-s337 and DFT-s446: a Vehicle crewed by Terrian,
// World Tyrant under March of the World Ooze. CR 613.8a: March's layer-4
// grant to "creatures you control" depends on crew's layer-4 "becomes an
// artifact creature" (applying crew changes what the grant applies to), so
// crew applies first even with the later timestamp and the Vehicle is an
// Ooze at 6/6. Forge and XMage apply the two in timestamp order and drop the
// Ooze type; gorge follows the rule. The first differing value in each
// window was the Vehicle's type line.
func TestFarmTerrianWorldTyrant(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		vehicle string
		crewIdx int // Rover Blades lists Equip before Crew
	}{
		{"Thundering Broodwagon", 0},
		{"Rover Blades", 1},
	} {
		vehicle, crewIdx := tc.vehicle, tc.crewIdx
		t.Run(vehicle, func(t *testing.T) {
			t.Parallel()
			scenario := `{
			  "name": "farm-terrian-crewed-vehicle-ooze",
			  "setup": {"p0": {"battlefield": ["` + vehicle + `", "Terrian, World Tyrant", "March of the World Ooze"]}},
			  "steps": [
			    {"op": "activate", "seat": 0, "card": "p0:` + vehicle + `", "answers": [{"kind": "choose", "pick": ["p0:Terrian, World Tyrant"]}], "ability_index": ` + strconv.Itoa(crewIdx) + `},
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
			for _, p := range last.Permanents {
				if p.Ref != "p0:"+vehicle {
					continue
				}
				types := map[string]bool{}
				for _, ty := range p.Types {
					types[ty] = true
				}
				if !types["Creature"] || !types["Vehicle"] || !types["Ooze"] || p.PT != "6/6" {
					t.Fatalf("crewed %s is %v %s, want an Ooze Vehicle creature 6/6", vehicle, p.Types, p.PT)
				}
				return
			}
			t.Fatalf("%s not on the battlefield: %+v", vehicle, last.Permanents)
		})
	}
}
