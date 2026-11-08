package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmFangGuardian pins the farm window DFT-s490 xe:RAND:2:2:w21, where
// Forge and XMage print the crewed Cryptcaller Chariot as [artifact creature
// vehicle] 6/6 and gorge prints [artifact creature ooze vehicle] 6/6.
//
// Gorge is right. March of the World Ooze makes "creatures you control" base
// 6/6 Oozes; crew's layer-4 "becomes an artifact creature" changes what that
// grant applies to, so the grant depends on it (CR 613.8a) and applies after
// it even though the crew effect has the later timestamp. Fang Guardian's ETB
// (+2/+2 on another creature or Vehicle) lands on Hazard of the Dunes in the
// window and does not touch the Chariot, which stays 6/6.
func TestFarmFangGuardian(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	scenario := `{
	  "name": "farm-fang-guardian",
	  "setup": {
	    "p0": {
	      "battlefield": ["Swamp", "Forest", "Forest", "Amonkhet Raceway", "Forest", "Swamp",
	        "Hazard of the Dunes", "Forest", "Forest", "March of the World Ooze", "Agonasaur Rex"],
	      "hand": ["Cryptcaller Chariot", "Fang Guardian", "Swamp"],
	      "counters": {"Hazard of the Dunes": {"P1P1": 3}},
	      "tokens": [{"script": "c_1_1_pilot_crewsaddlebuff"}],
	      "speed": 4
	    },
	    "p1": {"battlefield": ["Mountain"]}
	  },
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Amonkhet Raceway", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Forest#3", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Forest#4", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Forest#5", "ability_index": 0},
	    {"op": "cast", "seat": 0, "card": "p0:Fang Guardian"},
	    {"op": "pass", "seat": 0},
	    {"op": "resolve", "seat": 0, "answers": [{"kind": "target", "pick": ["p0:Hazard of the Dunes"]}]},
	    {"op": "play", "seat": 0, "card": "p0:Swamp#3"},
	    {"op": "activate", "seat": 0, "card": "p0:Forest", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Forest#2", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Swamp#2", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Swamp#3", "ability_index": 0},
	    {"op": "cast", "seat": 0, "card": "p0:Cryptcaller Chariot"},
	    {"op": "pass", "seat": 0},
	    {"op": "pass", "seat": 1},
	    {"op": "activate", "seat": 0, "card": "p0:Cryptcaller Chariot", "answers": [{"kind": "choose", "pick": ["p0:token:Pilot Token"]}], "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Cryptcaller Chariot", "answers": [{"kind": "choose", "pick": ["p0:Hazard of the Dunes"]}], "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Cryptcaller Chariot", "answers": [{"kind": "choose", "pick": ["p0:Agonasaur Rex"]}], "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Cryptcaller Chariot", "answers": [{"kind": "choose", "pick": ["p0:Fang Guardian"]}], "ability_index": 0},
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
		if p.Ref != "p0:Cryptcaller Chariot" {
			continue
		}
		got := strings.ToLower(strings.Join(p.Types, " "))
		// Precondition: the crew resolved, or the Ooze assertion is vacuous.
		if !strings.Contains(got, "creature") {
			t.Fatalf("Chariot is not a creature (crew did not resolve): %v %s", p.Types, p.PT)
		}
		if !strings.Contains(got, "ooze") || !strings.Contains(got, "vehicle") || p.PT != "6/6" {
			t.Fatalf("crewed Chariot is %v %s, want an Ooze Vehicle creature 6/6", p.Types, p.PT)
		}
		return
	}
	t.Fatalf("Cryptcaller Chariot not on the battlefield: %+v", last.Permanents)
}
