package rules

// CR 613.6: a static ability that began applying in layer 4 keeps applying in
// layer 7 to the set it affected then. Fowl Play and Frogify strip every card
// type and re-add Creature in their layer-4 part; the layer-7 base P/T part
// must still match the creature they enchanted even though its own type grant
// changed what Affected$ Creature reads.

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestLockedLayer4KeepsPriorTypes(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		card string
		mana string
		want string
	}{
		{"Fowl Play", "UUU", "1/1"},
		{"Frogify", "UU", "1/1"},
		{"Witness Protection", "U", "1/1"},
		{"Darksteel Mutation", "CW", "0/1"},
		{"Reprobation", "CW", "0/1"},
		{"Kasmina's Transmutation", "UU", "1/1"},
	} {
		t.Run(tc.card, func(t *testing.T) {
			t.Parallel()
			scenario := fmt.Sprintf(`{
			  "name": "locked-layer4-%s",
			  "setup": {"p0": {"battlefield": ["Craw Wurm"], "hand": ["%s"]}},
			  "steps": [
			    {"op": "cast", "seat": 0, "card": "p0:%s", "mana": "%s", "targets": ["p0:Craw Wurm"]},
			    {"op": "resolve"}
			  ],
			  "expect": [{"card": "p0:Craw Wurm", "zone": "battlefield", "pt": "%s"}]
			}`, tc.card, tc.card, tc.card, tc.mana, tc.want)
			res, err := RunOracleScenarioJSON(reg, []byte(scenario))
			if err != nil {
				t.Fatalf("%s: scenario JSON: %v", tc.card, err)
			}
			if len(res.Fails) != 0 {
				t.Fatalf("%s on Craw Wurm: %v", tc.card, res.Fails)
			}
			last := res.Snapshots[len(res.Snapshots)-1]
			found := false
			for _, p := range last.Permanents {
				if p.Ref != "p0:Craw Wurm" {
					continue
				}
				found = true
				if p.PT != tc.want {
					t.Fatalf("%s: Craw Wurm is %s, want %s", tc.card, p.PT, tc.want)
				}
			}
			if !found {
				t.Fatalf("%s: precondition failed, Craw Wurm is not on the battlefield: %+v", tc.card, last.Permanents)
			}
		})
	}
}

// CR 613.6: March of the Machines' layer-7 base P/T locks to the set its
// layer-4 grant matched, which keeps types the object already had. March
// alone gives 2/2 (Smuggler's Copter's mana value). With Aerial Modification
// attached, CR 613.8a makes March's `Artifact.nonCreature` layer-4 grant
// depend on Aerial's `Vehicle` grant: Aerial applies first, March's layer-4
// sibling never applies, so the layer-7 part is unlocked and evaluates
// against the finished types (Creature) and does not apply; Aerial's +2/+2
// on the printed 3/3 gives 5/5. That is Forge's order too
// (GameAction.findStaticAbilityToApply's dependency graph).
func TestLockedLayer4MarchKeepsPriorTypes(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name string
		ref  string
		json string
		want string
	}{
		{
			name: "March alone",
			ref:  "p0:Smuggler's Copter",
			json: `{
			  "name": "march-alone",
			  "setup": {"p0": {"battlefield": ["Smuggler's Copter", "March of the Machines"]}},
			  "steps": [{"op": "resolve"}],
			  "expect": [{"card": "p0:Smuggler's Copter", "zone": "battlefield", "pt": "2/2"}]
			}`,
			want: "2/2",
		},
		{
			name: "March then Aerial Modification",
			ref:  "p0:Smuggler's Copter",
			json: `{
			  "name": "march-aerial",
			  "setup": {"p0": {"battlefield": ["Smuggler's Copter", "March of the Machines"], "hand": ["Aerial Modification"]}},
			  "steps": [
			    {"op": "cast", "seat": 0, "card": "p0:Aerial Modification", "mana": "WWWWW", "targets": ["p0:Smuggler's Copter"]},
			    {"op": "resolve"}
			  ],
			  "expect": [{"card": "p0:Smuggler's Copter", "zone": "battlefield", "pt": "5/5"}]
			}`,
			want: "5/5",
		},
		{
			name: "Aerial Modification alone",
			ref:  "p0:Smuggler's Copter",
			json: `{
			  "name": "aerial-alone",
			  "setup": {"p0": {"battlefield": ["Smuggler's Copter"], "hand": ["Aerial Modification"]}},
			  "steps": [
			    {"op": "cast", "seat": 0, "card": "p0:Aerial Modification", "mana": "WWWWW", "targets": ["p0:Smuggler's Copter"]},
			    {"op": "resolve"}
			  ],
			  "expect": [{"card": "p0:Smuggler's Copter", "zone": "battlefield", "pt": "5/5"}]
			}`,
			want: "5/5",
		},
		{
			name: "Opalescence alone",
			ref:  "p0:March of the Machines",
			json: `{
			  "name": "opalescence-alone",
			  "setup": {"p0": {"battlefield": ["March of the Machines", "Opalescence"]}},
			  "steps": [{"op": "resolve"}],
			  "expect": [{"card": "p0:March of the Machines", "zone": "battlefield", "pt": "4/4"}]
			}`,
			want: "4/4",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res, err := RunOracleScenarioJSON(reg, []byte(tc.json))
			if err != nil {
				t.Fatalf("%s: scenario JSON: %v", tc.name, err)
			}
			if len(res.Fails) != 0 {
				t.Fatalf("%s: %v", tc.name, res.Fails)
			}
			last := res.Snapshots[len(res.Snapshots)-1]
			found := false
			for _, p := range last.Permanents {
				if p.Ref != tc.ref {
					continue
				}
				found = true
				if p.PT != tc.want {
					t.Fatalf("%s: %s is %s, want %s", tc.name, tc.ref, p.PT, tc.want)
				}
			}
			if !found {
				t.Fatalf("%s: precondition failed, %s is not on the battlefield: %+v", tc.name, tc.ref, last.Permanents)
			}
		})
	}
}
