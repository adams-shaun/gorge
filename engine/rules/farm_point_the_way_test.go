package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmPointtheWayVehicleIsOozeUnderMarch pins the value behind the DFT farm
// windows DFT-s503 xe:RAND:4:12:w16 and DFT-s456 xe:RAND:0:13:w16 (3way:FX|G,
// field permanents, attributed to Point the Way). The first differing value
// was the subtype line of a Vehicle that had become a creature while its
// controller had March of the World Ooze: gorge "artifact creature ooze
// vehicle 6/6", Forge and XMage "artifact creature vehicle 6/6".
//
// gorge is the CR-correct side. CR 613.8a: March's layer-4 "creatures you
// control are Oozes" depends on the Vehicle's layer-4 "becomes an artifact
// creature", because applying that effect changes what the grant applies to.
// CR 613.8b: the grant then applies after it, whatever the timestamps. Both
// followers apply the layer-7b 6/6 and drop the layer-4 Ooze. The farm's two
// Vehicles were crewed: Rangers' Refueler (s503) and Rover Blades (s456).
func TestFarmPointtheWayVehicleIsOozeUnderMarch(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const crew = `"answers": [{"kind": "choose", "pick": ["p0:Grizzly Bears"]}], "ability": "Crew"}`
	for _, tc := range []struct {
		name, vehicle, step, pt string
	}{
		// The farm's two windows: both Vehicles crewed, 6/6.
		{"Rangers' Refueler crewed", "Rangers' Refueler", `{"op": "activate", "seat": 0, "card": "p0:Rangers' Refueler", ` + crew, "6/6"},
		{"Rover Blades crewed", "Rover Blades", `{"op": "activate", "seat": 0, "card": "p0:Rover Blades", ` + crew, "6/6"},
		// Refueler's exhaust Animate (Duration$ Permanent) reaches the same
		// dependency through another layer-4 effect; its counter makes it 7/7.
		{"Rangers' Refueler exhausted", "Rangers' Refueler", `{"op": "activate", "seat": 0, "card": "p0:Rangers' Refueler", "mana": "CCCC", "ability": "+1/+1 counter"}, {"op": "resolve"}`, "7/7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			scenario := `{
			  "name": "farm-point-the-way-vehicle-ooze",
			  "setup": {"p0": {"battlefield": ["` + tc.vehicle + `", "Grizzly Bears", "March of the World Ooze", "Point the Way"]}},
			  "steps": [
			    ` + tc.step + `,
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
			// Precondition: before the activation the Vehicle is not a
			// creature, so March's grant does not reach it yet.
			first := res.Snapshots[0]
			for _, p := range first.Permanents {
				if p.Ref == "p0:"+tc.vehicle {
					for _, ty := range p.Types {
						if ty == "Creature" || ty == "Ooze" {
							t.Fatalf("precondition: %s is %v before activation, want a noncreature", tc.vehicle, p.Types)
						}
					}
				}
			}
			last := res.Snapshots[len(res.Snapshots)-1]
			for _, p := range last.Permanents {
				if p.Ref != "p0:"+tc.vehicle {
					continue
				}
				types := map[string]bool{}
				for _, ty := range p.Types {
					types[ty] = true
				}
				// The precondition the type check relies on: the effect
				// that animates it applied (Creature, and March's base 6/6).
				if !types["Creature"] || !types["Vehicle"] || p.PT != tc.pt {
					t.Fatalf("precondition: %s is %v %s, want an animated Vehicle %s", tc.vehicle, p.Types, p.PT, tc.pt)
				}
				if !types["Ooze"] {
					t.Fatalf("%s is %v %s, want an Ooze (CR 613.8a/b dependency)", tc.vehicle, p.Types, p.PT)
				}
				return
			}
			t.Fatalf("%s not on the battlefield: %+v", tc.vehicle, last.Permanents)
		})
	}
}
