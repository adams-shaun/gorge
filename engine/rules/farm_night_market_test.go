package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm finding DFT `permanents` (Night Market is only a co-occurring card in
// the windows; the diverging line is the crewed Vehicle). Forge and XMage
// print a crewed Lumbering Worldwagon under March of the World Ooze as
// `[artifact creature vehicle] 6/6`; gorge prints `[artifact creature ooze
// vehicle] 6/6`. March's layer-4 "creatures you control are Oozes" depends on
// crew's layer-4 "becomes an artifact creature" (CR 613.8a: applying crew
// changes the set of objects March applies to), so crew applies first even
// though March has the earlier timestamp, and the Vehicle is an Ooze. Forge
// and XMage apply layer 4 in plain timestamp order (March first, no creature
// yet) while still handing the Vehicle March's layer-7 6/6, which is the
// internally inconsistent `6/6` non-Ooze they print. gorge is the one that
// follows the rules, so this test pins gorge's value.
func TestFarmNightMarket(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name      string
		extra     string
		wantOoze  bool
		wantPower string
	}{
		{"march-on-battlefield", `, "March of the World Ooze"`, true, "6/6"},
		{"control-no-march", ``, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			scenario := `{
			  "name": "farm-night-market-crewed-ooze",
			  "setup": {"p0": {"battlefield": ["Lumbering Worldwagon", "Craw Wurm", "Night Market"` + tc.extra + `]}},
			  "steps": [
			    {"op": "activate", "seat": 0, "card": "p0:Lumbering Worldwagon", "answers": [{"kind": "choose", "pick": ["p0:Craw Wurm"]}], "ability_index": 0},
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
				// Precondition: the crew resolved, so the Vehicle is a creature.
				if !types["Creature"] || !types["Vehicle"] {
					t.Fatalf("Worldwagon is %v, want a crewed Vehicle creature", p.Types)
				}
				if types["Ooze"] != tc.wantOoze {
					t.Fatalf("crewed Worldwagon types %v, Ooze=%v want %v", p.Types, types["Ooze"], tc.wantOoze)
				}
				if tc.wantPower != "" && p.PT != tc.wantPower {
					t.Fatalf("crewed Worldwagon P/T %s, want %s", p.PT, tc.wantPower)
				}
				return
			}
			t.Fatalf("Worldwagon not on the battlefield: %+v", last.Permanents)
		})
	}
}
