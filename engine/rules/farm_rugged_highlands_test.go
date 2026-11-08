package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmRuggedHighlands replays the DFT-s122 game-17 window
// xe:RAND:2:17:w16. Guidelight Matrix animates Thunderous Velocipede into an
// artifact creature while March of the World Ooze is on the battlefield.
//
// Forge and XMage both print the animated Vehicle as a 6/6 artifact creature
// Vehicle with no Ooze type; gorge prints it as an Ooze. gorge is right:
// March's layer-4 "are Oozes" applies to "creatures you control", and the
// animation's layer-4 "becomes an artifact creature" changes what that set
// contains, so March depends on the animation and the animation applies first
// however the timestamps fall (CR 613.8a). March's layer-7 base P/T 6/6 then
// applies to the now-creature Vehicle, which is why both followers already
// print 6/6.
func TestFarmRuggedHighlands(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	scenario := `{
	  "name": "farm-rugged-highlands",
	  "cr": ["613.8a"],
	  "why": "DFT-s122 xe:RAND:2:17:w16: Guidelight Matrix animates a Vehicle under March of the World Ooze; the animation changes what March applies to, so March applies after it and the Vehicle is an Ooze (CR 613.8a)",
	  "setup": {"p0": {"battlefield": ["Forest", "Forest", "March of the World Ooze", "Thunderous Velocipede", "Guidelight Matrix"]},
	            "p1": {}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Forest", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Forest#2", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Guidelight Matrix", "targets": ["p0:Thunderous Velocipede"], "ability_index": 1},
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
	if len(last.Stack) != 0 {
		t.Fatalf("stack not drained, precondition for the check: %+v", last.Stack)
	}
	var matrixTapped, march bool
	var velocipede *OracleSnapPerm
	for i := range last.Permanents {
		switch last.Permanents[i].Ref {
		case "p0:Guidelight Matrix":
			matrixTapped = last.Permanents[i].Tapped
		case "p0:March of the World Ooze":
			march = true
		case "p0:Thunderous Velocipede":
			velocipede = &last.Permanents[i]
		}
	}
	if velocipede == nil {
		t.Fatalf("Thunderous Velocipede not on the battlefield (march=%v): %+v", march, last.Permanents)
	}
	if !matrixTapped {
		t.Fatalf("Guidelight Matrix is untapped, so the animate ability did not resolve")
	}
	// The precondition: the animation resolved (it is a creature) and March's
	// layer-7 base P/T reached it.
	for _, want := range []string{"Artifact", "Creature", "Vehicle"} {
		if !slices.Contains(velocipede.Types, want) {
			t.Fatalf("Thunderous Velocipede types = %v, missing %q: the animation did not resolve", velocipede.Types, want)
		}
	}
	if velocipede.PT != "6/6" {
		t.Fatalf("Thunderous Velocipede is %s, want 6/6: March's base P/T did not apply", velocipede.PT)
	}
	if !slices.Contains(velocipede.Types, "Ooze") {
		t.Fatalf("Thunderous Velocipede types = %v, missing Ooze (CR 613.8a: March depends on the animation)", velocipede.Types)
	}
}
