package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm DFT-s507 xe:RAND:0:0:w13 and DFT-s448 xe:RAND:3:7:w20 (3way:FX|G,
// permanents, attributed to Trip Up): under March of the World Ooze a crewed
// Vehicle reads "artifact creature ooze vehicle 6/6" in gorge and "artifact
// creature vehicle 6/6" in Forge and XMage. Gorge follows the CR: March's
// layer-4 "are Oozes" grant to creatures you control depends on crew's layer-4
// "becomes an artifact creature" (CR 613.8a(b): applying crew changes what the
// grant applies to), so it applies after crew despite crew's later timestamp
// (CR 613.8b) and the Vehicle is an Ooze. Forge and XMage apply layer 4 in
// timestamp order only, which leaves the Ooze type off while layer 7b still
// sees a creature (6/6). Trip Up resolving in the same window moves nothing on
// the Vehicle.
func TestFarmTripUpCrewedVehicleStaysOozeUnderMarch(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	scenario := `{
	  "name": "farm-trip-up-crewed-vehicle-ooze",
	  "setup": {"p0": {"battlefield": ["Rangers' Refueler", "Grizzly Bears", "Hill Giant", "March of the World Ooze"], "hand": ["Trip Up"], "mana": "UUUU"}, "p1": {}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Rangers' Refueler", "answers": [{"kind": "choose", "pick": ["p0:Grizzly Bears"]}], "ability_index": 1},
	    {"op": "pass", "seat": 0},
	    {"op": "pass", "seat": 1},
	    {"op": "cast", "seat": 0, "card": "p0:Trip Up", "mana": "UUUU", "targets": ["p0:Hill Giant"]},
	    {"op": "resolve", "answers": [{"kind": "choose", "pick": ["top"]}]}
	  ],
	  "expect": [{"card": "p0:Hill Giant", "zone": "library"}, {"stack_size": 0}]
	}`
	res, err := RunOracleScenarioJSON(reg, []byte(scenario))
	if err != nil {
		t.Fatalf("scenario JSON: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("%v\n%s", res.Fails, res.Transcript)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	for _, p := range last.Permanents {
		if p.Ref != "p0:Rangers' Refueler" {
			continue
		}
		types := map[string]bool{}
		for _, ty := range p.Types {
			types[ty] = true
		}
		if !types["Artifact"] || !types["Creature"] || !types["Vehicle"] {
			t.Fatalf("precondition: Refueler is %v, want a crewed artifact creature Vehicle", p.Types)
		}
		if !types["Ooze"] || p.PT != "6/6" {
			t.Fatalf("crewed Refueler under March is %v %s, want an Ooze 6/6 (CR 613.8a)", p.Types, p.PT)
		}
		return
	}
	t.Fatalf("Refueler not on the battlefield: %+v", last.Permanents)
}
