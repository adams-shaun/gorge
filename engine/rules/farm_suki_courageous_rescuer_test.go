package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm window class 3way:FX|G (TLA-s160/s278/s411/s487, field `stack`, Avatar's
// Wrath in the window): after p0's Avatar's Wrath airbends "all other
// creatures", Forge and XMage both hold "ability:Suki, Courageous Rescuer@p0"
// on the stack and gorge held nothing ("stack: gorge [] vs forge
// [ability:Suki, Courageous Rescuer@p0] @ step 8 (pass)").
//
// CR 603.10a: "Some zone-change triggers look back in time. These are
// leaves-the-battlefield abilities ..."; CR 603.6c: leaves-the-battlefield
// abilities trigger when a permanent moves from the battlefield to another
// zone. Suki is airbent in the same instruction as the other creatures, so she
// sees them leave (the 603.10a example: an artifact destroyed by the same
// sweep still triggers). gorge's api:Airbend exiled one object at a time, so
// Suki had already left when a later creature did; it now parks the departures
// as one batch (effects.effAirbend, as DestroyAll and ChangeZoneAll do).
func TestFarmSukiCourageousRescuer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	run := func(t *testing.T, scenario string) OracleSnapshot {
		t.Helper()
		res, err := RunOracleScenarioJSON(reg, []byte(scenario))
		if err != nil {
			t.Fatalf("scenario JSON: %v", err)
		}
		if len(res.Fails) != 0 {
			t.Fatalf("%v", res.Fails)
		}
		return res.Snapshots[len(res.Snapshots)-1]
	}
	hasSuki := func(s OracleSnapshot) bool {
		for _, st := range s.Stack {
			if st.Kind == "ability" && st.Source == "p0:Suki, Courageous Rescuer" {
				return true
			}
		}
		return false
	}
	t.Run("another permanent dying on your turn triggers", func(t *testing.T) {
		t.Parallel()
		s := run(t, `{"name": "suki-plain", "turn": 1,
		  "setup": {"p0": {"battlefield": ["Suki, Courageous Rescuer", "Grizzly Bears"]}, "p1": {}},
		  "steps": [{"op": "move", "seat": 0, "card": "p0:Grizzly Bears", "to": "graveyard"}]}`)
		if !hasSuki(s) {
			t.Fatalf("precondition: Suki's trigger is not on the stack after Bears died: %+v", s.Stack)
		}
	})
	t.Run("control: a resolving Doom Blade killing Bears triggers", func(t *testing.T) {
		t.Parallel()
		s := run(t, `{"name": "suki-doom-blade", "turn": 1,
		  "setup": {"p0": {"battlefield": ["Suki, Courageous Rescuer", "Grizzly Bears"], "hand": ["Doom Blade"], "mana": "CB"}, "p1": {}},
		  "steps": [
		    {"op": "cast", "seat": 0, "card": "p0:Doom Blade", "mana": "CB", "targets": ["p0:Grizzly Bears"]},
		    {"op": "pass", "seat": 0}, {"op": "pass", "seat": 1}]}`)
		if !hasSuki(s) {
			t.Fatalf("Suki's trigger is not on the stack after Doom Blade killed Bears: %+v", s.Stack)
		}
	})
	t.Run("Avatar's Wrath targeting Suki airbends the others", func(t *testing.T) {
		t.Parallel()
		s := run(t, `{"name": "suki-wrath-target-suki", "turn": 1,
		  "setup": {"p0": {"battlefield": ["Suki, Courageous Rescuer", "Grizzly Bears"], "hand": ["Avatar's Wrath"], "mana": "CCWW"}, "p1": {}},
		  "steps": [
		    {"op": "cast", "seat": 0, "card": "p0:Avatar's Wrath", "mana": "CCWW", "targets": ["p0:Suki, Courageous Rescuer"]},
		    {"op": "pass", "seat": 0}, {"op": "pass", "seat": 1}]}`)
		if !hasSuki(s) {
			t.Fatalf("Suki's trigger is not on the stack after Bears were airbent: %+v", s.Stack)
		}
	})
	t.Run("Avatar's Wrath with no target airbends Suki too", func(t *testing.T) {
		t.Parallel()
		s := run(t, `{"name": "suki-wrath-no-target", "turn": 1,
		  "setup": {"p0": {"battlefield": ["Suki, Courageous Rescuer", "Grizzly Bears"], "hand": ["Avatar's Wrath"], "mana": "CCWW"}, "p1": {}},
		  "steps": [
		    {"op": "cast", "seat": 0, "card": "p0:Avatar's Wrath", "mana": "CCWW"},
		    {"op": "pass", "seat": 0}, {"op": "pass", "seat": 1}]}`)
		if !hasSuki(s) {
			t.Fatalf("Suki's trigger is not on the stack after Suki and Bears were airbent together: %+v", s.Stack)
		}
	})
}
