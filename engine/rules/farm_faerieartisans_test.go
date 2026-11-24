package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm finding SUP `p0.exile` (xe:RAND:0:1:w9, Mirelurk Queen; also Rikku,
// River Kelpie): gorge exiled the nontoken creature that entered, Forge kept
// it beside its artifact token copy. Faerie Artisans' trigger is "create a
// token that's a copy of that creature ... Then exile all other tokens created
// with Faerie Artisans." CR 608.2c: the controller follows the instructions in
// the order written; the instructions exile only earlier tokens created with
// Faerie Artisans (its script's ImprintCards$ Remembered is the host's
// remembered tokens), never the entering card. The second cast checks the
// other half: the first token is exiled, the second token stays.
func TestFarmFaerieArtisans(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	scenario := `{
	  "name": "farm-faerie-artisans-keeps-entering-creature",
	  "setup": {"p0": {"battlefield": ["Forest","Forest","Forest","Forest"], "hand": ["Grizzly Bears","Grizzly Bears"]},
	    "p1": {"battlefield": ["Faerie Artisans"]}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Forest", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Forest#2", "ability_index": 0},
	    {"op": "cast", "seat": 0, "card": "p0:Grizzly Bears"},
	    {"op": "pass", "seat": 0},
	    {"op": "pass", "seat": 1},
	    {"op": "pass", "seat": 0},
	    {"op": "pass", "seat": 1},
	    {"op": "activate", "seat": 0, "card": "p0:Forest#3", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Forest#4", "ability_index": 0},
	    {"op": "cast", "seat": 0, "card": "p0:Grizzly Bears#2"},
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
	real, tokens := 0, 0
	for _, p := range last.Permanents {
		if p.Name != "Grizzly Bears" {
			continue
		}
		if p.Token {
			tokens++
		} else {
			real++
		}
	}
	if len(last.Stack) != 0 {
		t.Fatalf("precondition: stack not empty: %v", last.Stack)
	}
	if len(last.Players[1].Exile) != 0 || len(last.Players[0].Hand) != 0 {
		t.Fatalf("precondition: both Bears cast; p0 hand %v", last.Players[0].Hand)
	}
	if real != 2 || len(last.Players[0].Exile) != 0 {
		t.Fatalf("real Bears = %d, p0 exile = %v; want both real Bears on the battlefield, p0 exile empty", real, last.Players[0].Exile)
	}
	if tokens != 1 {
		t.Fatalf("token Bears = %d, want 1 (the first token exiled by the second trigger)", tokens)
	}
}
