package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm windows xe:RAND:3:13:w16 (DFT-s505) and xe:RAND:2:7:w20 (DFT-s461):
// March of the World Ooze is on the battlefield, a Vehicle is cast later and
// crewed by a summoning-sick Jibbirik Omnivore (a vanilla 3/2 -- the card the
// farm attributed the step to; it only supplies crew power). Forge and
// XMage print the crewed Vehicle as a 6/6 that is NOT an Ooze; gorge prints
// an Ooze Vehicle creature 6/6.
//
// Gorge is right. March's layer-4 "are Oozes" and crew's layer-4 "becomes an
// artifact creature" share a layer, and applying crew changes what March
// applies to, so March depends on crew (CR 613.8a) and crew is applied first
// although its timestamp is later (CR 613.8b overrides timestamp order). With
// crew applied the Vehicle is a creature when March's layer-4 and layer-7b
// parts select their objects, and CR 613.6 keeps that one set across both
// layers: it is an Ooze and 6/6 together, never 6/6 without Ooze.
func TestFarmJibbirikOmnivore(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		vehicle string
		mana    string
		lands   string
	}{
		// DFT-s505: Cryptcaller Chariot {3}{B} (5/5, Crew 2).
		{"Cryptcaller Chariot", "BBGG", `"Swamp", "Swamp", "Forest", "Forest"`},
		// DFT-s461: Burner Rocket {1}{R} (3/1, Crew 1); its enters trigger
		// has Jibbirik Omnivore as its only legal target (the Vehicle is not a
		// creature yet), so the harness picks it.
		{"Burner Rocket", "RR", `"Mountain", "Mountain"`},
	} {
		t.Run(tc.vehicle, func(t *testing.T) {
			t.Parallel()
			scenario := fmt.Sprintf(`{
			  "name": "farm-jibbirik-omnivore",
			  "setup": {"p0": {"battlefield": ["Jibbirik Omnivore", "March of the World Ooze", %s], "hand": ["%s"]}},
			  "steps": [
			    {"op": "cast", "seat": 0, "card": "p0:%s", "mana": "%s"},
			    {"op": "pass", "seat": 0},
			    {"op": "pass", "seat": 1},
			    {"op": "activate", "seat": 0, "card": "p0:%s", "answers": [{"kind": "choose", "pick": ["p0:Jibbirik Omnivore"]}], "ability_index": 0},
			    {"op": "pass", "seat": 0},
			    {"op": "pass", "seat": 1}
			  ]
			}`, tc.lands, tc.vehicle, tc.vehicle, tc.mana, tc.vehicle)
			res, err := RunOracleScenarioJSON(reg, []byte(scenario))
			if err != nil {
				t.Fatalf("scenario JSON: %v", err)
			}
			if len(res.Fails) != 0 {
				t.Fatalf("%v", res.Fails)
			}
			last := res.Snapshots[len(res.Snapshots)-1]
			seen := false
			for _, p := range last.Permanents {
				if p.Ref != "p0:"+tc.vehicle {
					continue
				}
				seen = true
				types := map[string]bool{}
				for _, ty := range p.Types {
					types[ty] = true
				}
				// Preconditions: the crew resolved (the Vehicle is a creature)
				// and March is what makes it 6/6, not its printed size.
				if !types["Creature"] || !types["Vehicle"] {
					t.Fatalf("%s is %v %s, want a crewed Vehicle creature: crew did not resolve", tc.vehicle, p.Types, p.PT)
				}
				if !types["Ooze"] || p.PT != "6/6" {
					t.Fatalf("crewed %s is %v %s, want an Ooze 6/6 (CR 613.8a: March depends on crew)", tc.vehicle, p.Types, p.PT)
				}
			}
			if !seen {
				t.Fatalf("%s not on the battlefield: %+v", tc.vehicle, last.Permanents)
			}
		})
	}
}
