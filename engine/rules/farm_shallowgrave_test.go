package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm finding SUP `p0.graveyard` (SUP-s351-supp xe:RAND:2:2:w17): Forge
// (and XMage's ShallowGraveEffect, which keeps the last creature card of the
// graveyard) returns the top creature card of the graveyard; gorge returned
// nothing. Shallow Grave: "Return the top creature card of your graveyard to
// the battlefield. That creature gains haste until end of turn." CR 404.1 puts
// each card on top of its owner's graveyard and CR 404.2 keeps the graveyard a
// single ordered pile, so the top creature card is well defined even when a
// non-creature card sits above it. The script selects with
// Creature.TopGraveyardCreature+YouCtrl.
func TestFarmShallowGrave(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	scenario := `{
	  "name": "farm-shallow-grave-top-creature",
	  "setup": {"p0": {"battlefield": ["Swamp","Swamp"], "hand": ["Shallow Grave"],
	    "graveyard": ["Grizzly Bears","Hill Giant","Forest"]}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Swamp", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Swamp#2", "ability_index": 0},
	    {"op": "cast", "seat": 0, "card": "p0:Shallow Grave"},
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
	first := res.Snapshots[0]
	if !slices.Contains(first.Players[0].Graveyard, "Hill Giant") {
		t.Fatalf("precondition: graveyard = %v, want the scripted cards", first.Players[0].Graveyard)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	p0 := last.Players[0]
	if !slices.Contains(p0.Graveyard, "Shallow Grave") {
		t.Fatalf("precondition: Shallow Grave did not resolve; graveyard = %v", p0.Graveyard)
	}
	var returned []string
	haste := false
	for _, p := range last.Permanents {
		if p.Name == "Grizzly Bears" || p.Name == "Hill Giant" {
			returned = append(returned, p.Name)
			haste = slices.Contains(p.Keywords, "Haste")
		}
	}
	t.Logf("first graveyard %v, last graveyard %v, returned %v", first.Players[0].Graveyard, p0.Graveyard, returned)
	if len(returned) != 1 {
		t.Fatalf("returned creatures = %v (graveyard %v), want exactly the top creature card", returned, p0.Graveyard)
	}
	// The graveyard lists bottom first: Hill Giant is the topmost creature
	// card, under Forest; Grizzly Bears below it stays.
	if returned[0] != "Hill Giant" {
		t.Fatalf("returned %v, want the topmost creature card Hill Giant", returned)
	}
	if !slices.Equal(p0.Graveyard, []string{"Grizzly Bears", "Forest", "Shallow Grave"}) {
		t.Fatalf("graveyard = %v, want [Grizzly Bears Forest Shallow Grave]", p0.Graveyard)
	}
	if !haste {
		t.Fatalf("Hill Giant returned without haste")
	}
}
