package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmVeteranBeastrider replays the DFT-s446 game-11 window (farm rows
// xe:RAND:3:11:w20 / w22): Veloheart Bike is crewed by Veteran Beastrider under
// March of the World Ooze, then two of Beastrider's "{2}{G}{W}: creatures you
// control get +1/+1" activations are stacked above the crew and resolve first.
//
// Forge and XMage print the Bike as a 6/6 artifact creature Vehicle with no
// Ooze type; gorge prints it as an Ooze. gorge is right: March's layer-4
// "are Oozes" applies to "creatures you control", and crew's layer-4 "becomes
// an artifact creature" changes what that set contains, so March depends on
// crew and crew applies first however the timestamps fall (CR 613.8a). Both
// pumps resolve while the Bike is still not a creature (CR 611.2c: PumpAll
// locks its set at resolution), so it stays 6/6.
func TestFarmVeteranBeastrider(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	scenario := `{
	  "name": "farm-veteran-beastrider",
	  "setup": {"p0": {"battlefield": ["Forest", "Forest", "Plains", "Veteran Beastrider", "Plains", "Forest", "Forest", "Plains", "Veloheart Bike", "Plains", "March of the World Ooze", "Forest"]}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Veloheart Bike", "answers": [{"kind": "choose", "pick": ["p0:Veteran Beastrider"]}], "ability_index": 1},
	    {"op": "activate", "seat": 0, "card": "p0:Forest#4", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Plains#3", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Plains#4", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Forest#5", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Veteran Beastrider", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Forest#2", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Plains", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Plains#2", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Forest#3", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Veteran Beastrider", "ability_index": 0},
	    {"op": "pass", "seat": 0},
	    {"op": "pass", "seat": 1},
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
	if len(last.Stack) != 0 {
		t.Fatalf("stack not drained, precondition for the check: %+v", last.Stack)
	}
	var bike, rider bool
	for _, p := range last.Permanents {
		switch p.Ref {
		case "p0:Veteran Beastrider":
			// The crew cost tapped it; it is a creature the whole time, so
			// both pumps reached it: base 3/4 under March's 6/6 setting
			// plus two +1/+1.
			rider = true
			if p.PT != "8/8" {
				t.Fatalf("Veteran Beastrider is %s, want 8/8 (March 6/6 + two pumps): pumps did not both resolve", p.PT)
			}
		case "p0:Veloheart Bike":
			bike = true
			for _, want := range []string{"Artifact", "Creature", "Vehicle", "Ooze"} {
				if !slices.Contains(p.Types, want) {
					t.Fatalf("crewed Veloheart Bike types = %v, missing %q (CR 613.8a: March depends on crew)", p.Types, want)
				}
			}
			if p.PT != "6/6" {
				t.Fatalf("crewed Veloheart Bike is %s, want 6/6 (both pumps resolved before the crew made it a creature, CR 611.2c)", p.PT)
			}
		}
	}
	if !bike || !rider {
		t.Fatalf("bike=%v rider=%v on the battlefield: %+v", bike, rider, last.Permanents)
	}
}
