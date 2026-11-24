package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm finding SUP `p0.library_top` (xe:RAND:4:17:w6 and others; XMage and
// Forge both corroborate): Intuition ends "Then shuffle." Its last sub-ability
// is a ChangeZoneAll from the library to the graveyard with Shuffle$ True, and
// CR 701.24a ("To shuffle a library ..., randomize the cards within it") with
// CR 608.2c (follow the instructions in the order written) has the library
// randomized after the cards move. gorge ran the ChangeZoneAll shuffle tail only
// when the DESTINATION was the library, so Intuition left the library in setup
// order and drew no RNG.
func TestFarmIntuition(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	scenario := `{
	  "name": "farm-intuition-then-shuffle",
	  "setup": {"p0": {"battlefield": ["Island","Island","Island"], "hand": ["Intuition"],
	    "library_top": ["Island","Plains","Mountain","Forest","Swamp","Lightning Bolt","Grizzly Bears","Shock"]}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Island", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Island#2", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Island#3", "ability_index": 0},
	    {"op": "cast", "seat": 0, "card": "p0:Intuition"},
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
	p0 := last.Players[0]
	if !slices.Contains(p0.Graveyard, "Intuition") || len(p0.Graveyard) != 3 {
		t.Fatalf("precondition: Intuition resolved and put two searched cards in the graveyard; graveyard = %v", p0.Graveyard)
	}
	if len(p0.LibraryTop) < 5 {
		t.Fatalf("precondition: p0 library = %v, want the five unsearched cards", p0.LibraryTop)
	}
	setup := []string{"Island", "Plains", "Mountain", "Forest", "Swamp", "Lightning Bolt", "Grizzly Bears", "Shock"}
	// The unsearched remainder in setup order: what an unshuffled library holds.
	var rest []string
	for _, n := range setup {
		if i := slices.Index(p0.Graveyard, n); i >= 0 {
			continue
		}
		if slices.Contains(p0.Hand, n) {
			continue
		}
		rest = append(rest, n)
	}
	if slices.Equal(p0.LibraryTop[:len(rest)], rest) {
		t.Fatalf("p0 library = %v is still in setup order: Intuition's \"Then shuffle.\" did not shuffle (CR 701.24a)", p0.LibraryTop)
	}
}
