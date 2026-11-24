package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm finding SUP `p0.life` (SUP-s350-supp / SUP-s354-supp): gorge left both
// players at 16 after Mind Bomb with no discards, Forge (the corroborating
// engine) at 17. Mind Bomb: "Each player may discard up to three cards. Mind
// Bomb deals damage to each player equal to 3 minus the number of cards they
// discarded this way." CR 608.2c: the controller follows the instructions in
// the order written, so with zero discards each player takes exactly 3. The
// script computes 3 - (Count$RememberedSize - 1), where the Minus.1 removes
// the RepeatEach player sitting in the host's remembered list beside the
// discarded cards.
func TestFarmMindBomb(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	scenario := `{
	  "name": "farm-mind-bomb-no-discard",
	  "setup": {"p0": {"hand": ["Mind Bomb","Wastes","Wastes"]}, "p1": {"hand": ["Wastes"]}},
	  "steps": [
	    {"op": "cast", "seat": 0, "card": "p0:Mind Bomb", "mana": "U"},
	    {"op": "resolve", "seat": 0}
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
	if len(last.Players[0].Hand) != 2 || len(last.Players[1].Hand) != 1 {
		t.Fatalf("precondition: someone discarded (p0 hand %v, p1 hand %v)", last.Players[0].Hand, last.Players[1].Hand)
	}
	if !slices.Contains(last.Players[0].Graveyard, "Mind Bomb") {
		t.Fatalf("precondition: Mind Bomb did not resolve (p0 graveyard %v)", last.Players[0].Graveyard)
	}
	for i, p := range last.Players {
		if p.Life != 17 {
			t.Errorf("p%d life = %d, want 17 (20 - (3 - 0 discarded))", i, p.Life)
		}
	}
}
