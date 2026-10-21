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
