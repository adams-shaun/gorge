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

// TestStackZoneCastTriggerSeesOnlyItsOwnCast pins Storied Veteran's "You may
// cast this card from your graveyard. If you do, it enters with a finality
// counter on it" to its own cast. Its SpellCast trigger functions on the
// stack (TriggerZones$ Stack) and names the cast as the bare ValidCard$ Card;
// read as any spell, a Spectral Sailor cast from the top of the library
// through Future Sight while the Veteran waits on the stack got the Veteran's
// finality counter too.
func TestStackZoneCastTriggerSeesOnlyItsOwnCast(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"x","cr":["603.2","614.12"],"why":"x","setup":{"p0":{"battlefield":["Future Sight"],"library_top":["Spectral Sailor","Spectral Sailor"],"graveyard":["Storied Veteran"]},"p1":{"battlefield":["Grizzly Bears"]}},"steps":[
		{"op":"pass_to","seat":0,"step":"main1","decision":"priority"},
		{"op":"cast","seat":0,"card":"p0:Storied Veteran","mana":"B"},
		{"op":"cast","seat":0,"card":"p0:Spectral Sailor","mana":"U"},
		{"op":"resolve","seat":0},{"op":"resolve","seat":0}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil || len(res.Fails) != 0 {
		t.Fatalf("scenario: err=%v fails=%v", err, res.Fails)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	got := map[string]int32{}
	seen := map[string]bool{}
	for _, p := range last.Permanents {
		if p.Name == "Storied Veteran" || p.Name == "Spectral Sailor" {
			got[p.Name] += p.Counters["FINALITY"]
			seen[p.Name] = true
		}
	}
	if !seen["Storied Veteran"] || !seen["Spectral Sailor"] {
		t.Fatalf("both should be on the battlefield: %+v", last.Permanents)
	}
	if got["Storied Veteran"] != 1 || got["Spectral Sailor"] != 0 {
		t.Fatalf("FINALITY counters %v, want Storied Veteran 1 and Spectral Sailor 0", got)
	}
}
