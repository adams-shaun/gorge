package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// farmOozeVehicle runs a scenario with March of the World Ooze already in play
// and returns the named permanent's types and P/T after the last step.
func farmOozeVehicle(t *testing.T, scenario, ref string) ([]string, string) {
	t.Helper()
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatalf("scenario JSON: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("%v", res.Fails)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	for _, p := range last.Permanents {
		if p.Ref == ref {
			return p.Types, p.PT
		}
	}
	t.Fatalf("%s not on the battlefield: %+v", ref, last.Permanents)
	return nil, ""
}

// Farm DFT-s110 xe:RAND:1:5:w15 / DFT-s458 xe:RAND:0:4:w15 (field permanents):
// gorge showed "Debris Beetle [artifact creature ooze vehicle] 6/6" where Forge
// and XMage showed "[artifact creature vehicle] 6/6". CR 613.8a: March of the
// World Ooze's layer-4 "creatures you control are Oozes" depends on the
// layer-4 effect that makes a Vehicle a creature (its application changes what
// the March effect applies to), so the Vehicle-turned-creature is applied first
// whatever the timestamps and is an Ooze. Forge and XMage apply the 6/6 base
// P/T (layer 7b) but miss the Ooze type; gorge is right, so this pins gorge.
func TestFarmJungleHollowGuidelightAnimatedVehicleIsOoze(t *testing.T) {
	t.Parallel()
	types, pt := farmOozeVehicle(t, `{
	  "name": "farm-jungle-hollow-animate",
	  "setup": {"p0": {"battlefield": ["March of the World Ooze", "Debris Beetle", "Guidelight Matrix", "Jungle Hollow", "Jungle Hollow"]}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Guidelight Matrix", "ability_index": 1, "mana": "BG", "targets": ["p0:Debris Beetle"]},
	    {"op": "pass", "seat": 0},
	    {"op": "pass", "seat": 1}
	  ]
	}`, "p0:Debris Beetle")
	if !slices.Contains(types, "Creature") {
		t.Fatalf("precondition: animated Beetle must be a creature, types = %v", types)
	}
	for _, want := range []string{"Artifact", "Vehicle", "Ooze"} {
		if !slices.Contains(types, want) {
			t.Fatalf("animated Beetle types = %v, missing %s (CR 613.8a)", types, want)
		}
	}
	if pt != "6/6" {
		t.Fatalf("animated Beetle P/T = %s, want 6/6", pt)
	}
}

// The crew route of the same window (DFT-s458 Rover Blades): a crewed 2/2
// equipment Vehicle is a 6/6 Ooze under March of the World Ooze.
func TestFarmJungleHollowCrewedRoverBladesIsOoze(t *testing.T) {
	t.Parallel()
	types, pt := farmOozeVehicle(t, `{
	  "name": "farm-jungle-hollow-crew",
	  "setup": {"p0": {"battlefield": ["March of the World Ooze", "Rover Blades", "Grizzly Bears"]}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Rover Blades", "answers": [{"kind": "choose", "pick": ["p0:Grizzly Bears"]}], "ability": "Crew"},
	    {"op": "pass", "seat": 0},
	    {"op": "pass", "seat": 1}
	  ]
	}`, "p0:Rover Blades")
	if !slices.Contains(types, "Creature") {
		t.Fatalf("precondition: crewed Rover Blades must be a creature, types = %v", types)
	}
	for _, want := range []string{"Artifact", "Equipment", "Vehicle", "Ooze"} {
		if !slices.Contains(types, want) {
			t.Fatalf("crewed Rover Blades types = %v, missing %s (CR 613.8a)", types, want)
		}
	}
	if pt != "6/6" {
		t.Fatalf("crewed Rover Blades P/T = %s, want 6/6", pt)
	}
}
