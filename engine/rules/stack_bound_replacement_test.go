package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestStackBoundEntryRiderEndsWithTheSpell pins CR 400.7 for the
// "if you cast it this way, it enters with a finality counter" riders: the
// Effect the cast's trigger creates is about that one spell, so it ends once
// the spell leaves the stack. Storied Veteran cast from the graveyard enters
// with one FINALITY counter; bounced and recast from hand it is a new object
// and gets none. Countered and recast from the graveyard, the second cast's
// own rider gives exactly one counter, not one per earlier cast.
func TestStackBoundEntryRiderEndsWithTheSpell(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name, scenario string
		want           []int32 // FINALITY on Storied Veteran at each battlefield sighting
	}{
		{"bounce and recast from hand", `{"name":"x","cr":["400.7","614.12"],"why":"x","setup":{"p0":{"hand":["Unsummon"],"graveyard":["Storied Veteran"]},"p1":{"battlefield":["Grizzly Bears"]}},"steps":[
			{"op":"pass_to","seat":0,"step":"main1","decision":"priority"},
			{"op":"cast","seat":0,"card":"p0:Storied Veteran","mana":"B"},{"op":"resolve","seat":0},
			{"op":"cast","seat":0,"card":"p0:Unsummon","mana":"U","targets":["p0:Storied Veteran"]},{"op":"resolve","seat":0},
			{"op":"cast","seat":0,"card":"p0:Storied Veteran","mana":"B"},{"op":"resolve","seat":0}]}`, []int32{1, 0}},
		{"countered and recast from the graveyard", `{"name":"x","cr":["400.7","614.12"],"why":"x","setup":{"p0":{"graveyard":["Storied Veteran"]},"p1":{"hand":["Counterspell"],"battlefield":["Grizzly Bears"]}},"steps":[
			{"op":"pass_to","seat":0,"step":"main1","decision":"priority"},
			{"op":"cast","seat":0,"card":"p0:Storied Veteran","mana":"B"},{"op":"pass","seat":0},
			{"op":"cast","seat":1,"card":"p1:Counterspell","targets":["p0:Storied Veteran"],"mana":"UU"},{"op":"resolve"},
			{"op":"cast","seat":0,"card":"p0:Storied Veteran","mana":"B"},{"op":"resolve","seat":0}]}`, []int32{1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := RunOracleScenarioJSON(reg, []byte(tc.scenario))
			if err != nil || len(res.Fails) != 0 {
				t.Fatalf("scenario: err=%v fails=%v", err, res.Fails)
			}
			// one entry per stint on the battlefield: a sighting after an
			// absence starts a new stint
			var got []int32
			on := false
			for _, s := range res.Snapshots {
				seen := false
				for _, p := range s.Permanents {
					if p.Name == "Storied Veteran" {
						seen = true
						if !on {
							got = append(got, p.Counters["FINALITY"])
						}
					}
				}
				on = seen
			}
			if len(got) != len(tc.want) {
				t.Fatalf("Storied Veteran stints %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("FINALITY per stint %v, want %v", got, tc.want)
				}
			}
		})
	}
}
