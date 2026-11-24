package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm finding SUP `p0.graveyard` (SUP-s338-supp xe:RAND:2:11:w12,
// xe:RAND:4:7:w13, xe:RAND:2:4:w13): Celestial Judgment reads "For each
// different power among creatures on the battlefield, choose a creature with
// that power. Destroy each creature not chosen this way." CR 608.2c: the
// controller follows the instructions in the order written, so with creatures
// of N distinct powers exactly N creatures are chosen and only the surplus
// same-power creatures are destroyed. Forge (ChooseCardEffect's
// EachDifferentPower$ arm: one pick per distinct power, then DestroyAll
// Creature.nonChosenCard) corroborates; gorge offered one optional pick over
// every creature and destroyed all the rest.
func TestFarmCelestialJudgment(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	scenario := `{
	  "name": "farm-celestial-judgment-each-different-power",
	  "setup": {"p0": {"battlefield": ["Plains","Plains","Plains","Plains","Plains","Plains","Grizzly Bears","Hill Giant"], "hand": ["Celestial Judgment"]},
	    "p1": {"battlefield": ["Runeclaw Bear"]}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Plains", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Plains#2", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Plains#3", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Plains#4", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Plains#5", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Plains#6", "ability_index": 0},
	    {"op": "cast", "seat": 0, "card": "p0:Celestial Judgment"},
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
	creatures := map[string]bool{"Grizzly Bears": true, "Hill Giant": true, "Runeclaw Bear": true}
	resolved := false
	destroyed := 0
	var graves [][]string
	for _, p := range last.Players {
		graves = append(graves, p.Graveyard)
		for _, n := range p.Graveyard {
			if n == "Celestial Judgment" {
				resolved = true
			}
			if creatures[n] {
				destroyed++
			}
		}
	}
	if !resolved {
		t.Fatalf("precondition: Celestial Judgment did not resolve (graveyards %v)", graves)
	}
	survivors := map[string]bool{}
	for _, p := range last.Permanents {
		if creatures[p.Name] {
			survivors[p.Name] = true
		}
	}
	if destroyed != 1 || !survivors["Hill Giant"] {
		t.Fatalf("creatures destroyed = %d (graveyards %v, surviving %v), want 1: one creature kept per distinct power (CR 608.2c), Hill Giant the only power 3", destroyed, graves, survivors)
	}
}
