package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 303.4f, 603.6a: an Aura enters attached, so its static ability applies
// before triggers are collected. An Aura that removes all abilities silences
// its host's "whenever an enchantment you control enters"; one that leaves
// abilities alone does not.
func TestAuraLossSilencesHostEerieTrigger(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		aura      string
		mana      string
		wantStack int
	}{
		{"Unable to Scream", "U", 0},
		{"Pacifism", "WW", 1},
	} {
		t.Run(tc.aura, func(t *testing.T) {
			t.Parallel()
			scenario := `{
			  "name": "aura-loss-host-eerie",
			  "setup": {"p0": {"battlefield": ["Stalked Researcher"], "hand": ["` + tc.aura + `"]}},
			  "steps": [
			    {"op": "cast", "seat": 0, "card": "p0:` + tc.aura + `", "mana": "` + tc.mana + `", "targets": ["p0:Stalked Researcher"]},
			    {"op": "pass", "seat": 0},
			    {"op": "pass", "seat": 1}
			  ]
			}`
			res, err := RunOracleScenarioJSON(reg, []byte(scenario))
			if err != nil {
				t.Fatalf("scenario JSON: %v", err)
			}
			if len(res.Fails) != 0 {
				t.Fatalf("%s: %v", tc.aura, res.Fails)
			}
			last := res.Snapshots[len(res.Snapshots)-1]
			if got := len(last.Stack); got != tc.wantStack {
				t.Fatalf("%s: stack has %d entries, want %d: %+v", tc.aura, got, tc.wantStack, last.Stack)
			}
		})
	}
}

// CR 613.8a: March of the World Ooze's layer-4 grant to "creatures you
// control" depends on crew's layer-4 "becomes an artifact creature", whose
// application changes what the grant applies to, so crew applies first even
// though its timestamp is later and the crewed Vehicle is an Ooze.
func TestCrewedVehicleIsOozeUnderMarchOfTheWorldOoze(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	scenario := `{
	  "name": "crewed-vehicle-ooze",
	  "setup": {"p0": {"battlefield": ["Lumbering Worldwagon", "Grizzly Bears", "March of the World Ooze"]}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Lumbering Worldwagon", "answers": [{"kind": "choose", "pick": ["p0:Grizzly Bears"]}], "ability_index": 0},
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
		if p.Ref != "p0:Lumbering Worldwagon" {
			continue
		}
		types := map[string]bool{}
		for _, ty := range p.Types {
			types[ty] = true
		}
		if !types["Creature"] || !types["Ooze"] || !types["Vehicle"] || p.PT != "6/6" {
			t.Fatalf("crewed Worldwagon is %v %s, want an Ooze Vehicle creature 6/6", p.Types, p.PT)
		}
		return
	}
	t.Fatalf("Worldwagon not on the battlefield: %+v", last.Permanents)
}
