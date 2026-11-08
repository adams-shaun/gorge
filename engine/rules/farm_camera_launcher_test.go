package rules

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmCameraLauncher pins the first differing value in the three DFT farm
// windows that attribute Camera Launcher: a Vehicle under March of the World
// Ooze is a 6/6 Ooze in addition to its other types once the Vehicle is
// animated. The farm pair reads "gorge ... [artifact creature ooze vehicle]
// 6/6" against "forge ... [artifact creature vehicle] 6/6" (and XMage the
// same), because both other engines fold layer-4 type records in raw
// timestamp order; CR 613.8a makes March's grant depend on the animation --
// applying the animation changes which objects "creatures you control"
// covers -- so the animation applies first and gorge's value is the right
// one. See gorge-compliance docs/upstream-bugs.md, "March of the World Ooze
// over a Vehicle animated after it entered".
//
// The scenario lists March before the Vehicle, so its layer-4 effect is the
// older one and timestamp order alone would miss the Ooze; that is exactly
// the farm ordering and the value the two other engines print.
func TestFarmCameraLauncher(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)

	for _, tc := range []struct {
		name    string
		vehicle string
		crew    []string
		color   string
	}{
		// DFT-s640: Camera Launcher and friends crew a Hulldrifter.
		{"Hulldrifter", "Hulldrifter", []string{"p0:Camera Launcher", "p0:Grizzly Bears"}, "U"},
		// DFT-s128: Camera Launcher and friends crew a Lumbering Worldwagon.
		{"Lumbering Worldwagon", "Lumbering Worldwagon", []string{"p0:Camera Launcher", "p0:Grizzly Bears"}, "G"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sc := map[string]any{
				"name": t.Name(), "turn": 1,
				"why": "CR 613.8a: March of the World Ooze's layer-4 Ooze grant depends on crew's animation and applies after it",
				"setup": map[string]any{
					"p0": map[string]any{
						"battlefield": []string{"March of the World Ooze", tc.vehicle, "Camera Launcher", "Grizzly Bears"},
					},
					"p1": map[string]any{},
				},
				"steps": []map[string]any{
					{"op": "activate", "seat": 0, "card": "p0:" + tc.vehicle, "ability_index": 0,
						"answers": []map[string]any{{"kind": "choose", "pick": tc.crew}}},
					{"op": "pass", "seat": 0},
					{"op": "pass", "seat": 1},
				},
			}
			raw, err := json.Marshal(sc)
			if err != nil {
				t.Fatal(err)
			}
			res, err := RunOracleScenarioJSON(reg, raw)
			if err != nil {
				t.Fatalf("scenario JSON: %v", err)
			}
			if len(res.Fails) != 0 {
				t.Fatalf("%v", res.Fails)
			}

			// Precondition: before the crew the Vehicle is a non-creature, so
			// the Ooze value can only come from the animation the crew applies.
			setup := res.Snapshots[0]
			before := farmPermanent(t, setup, tc.vehicle)
			if !slices.Contains(before.Types, "Vehicle") || slices.Contains(before.Types, "Creature") || slices.Contains(before.Types, "Ooze") {
				t.Fatalf("setup: %s types = %v, want a non-creature Vehicle without Ooze", tc.vehicle, before.Types)
			}
			if !farmPermanentOnBoard(setup, "March of the World Ooze") {
				t.Fatalf("precondition: March of the World Ooze is not on the battlefield at setup: %+v", setup.Permanents)
			}

			// The first differing value: the animated, crewed Vehicle is a
			// 6/6 Ooze Vehicle creature, not merely a 6/6 Vehicle creature.
			last := res.Snapshots[len(res.Snapshots)-1]
			got := farmPermanent(t, last, tc.vehicle)
			if !slices.Equal(got.Types, []string{"Artifact", "Creature", "Ooze", "Vehicle"}) {
				t.Fatalf("%s types = %v, want [Artifact Creature Ooze Vehicle] (CR 613.8a: the animation applies before March's grant)", tc.vehicle, got.Types)
			}
			if got.Colors != tc.color {
				t.Fatalf("%s colors = %q, want %q", tc.vehicle, got.Colors, tc.color)
			}
			if got.PT != "6/6" {
				t.Fatalf("%s P/T = %q, want 6/6", tc.vehicle, got.PT)
			}
		})
	}

	// DFT-s109: the window opens with both March and a self-animating Vehicle
	// already on the battlefield ("during turns other than yours"), so the
	// first differing value is the setup snapshot itself. Both permanents are
	// p1's and it is p0's turn.
	t.Run("Midnight Mangler at setup", func(t *testing.T) {
		t.Parallel()
		sc := map[string]any{
			"name": t.Name(), "turn": 1,
			"why": "CR 613.8a: March of the World Ooze entered first; Midnight Mangler's own animation still applies before the Ooze grant",
			"setup": map[string]any{
				"p0": map[string]any{},
				"p1": map[string]any{
					"battlefield": []string{"March of the World Ooze", "Midnight Mangler"},
				},
			},
			"steps": []map[string]any{},
		}
		raw, err := json.Marshal(sc)
		if err != nil {
			t.Fatal(err)
		}
		res, err := RunOracleScenarioJSON(reg, raw)
		if err != nil {
			t.Fatalf("scenario JSON: %v", err)
		}
		if len(res.Fails) != 0 {
			t.Fatalf("%v", res.Fails)
		}
		setup := res.Snapshots[0]
		if setup.Active != 0 {
			t.Fatalf("precondition: active seat = %d, want p0's turn so Midnight Mangler's not-your-turn animation is live", setup.Active)
		}
		got := farmPermanent(t, setup, "Midnight Mangler")
		if got.Controller != 1 {
			t.Fatalf("precondition: Midnight Mangler controller = %d, want p1", got.Controller)
		}
		if !slices.Equal(got.Types, []string{"Artifact", "Creature", "Ooze", "Vehicle"}) {
			t.Fatalf("Midnight Mangler types = %v, want [Artifact Creature Ooze Vehicle] (CR 613.8a: its animation applies before March's grant)", got.Types)
		}
		if got.Colors != "U" {
			t.Fatalf("Midnight Mangler colors = %q, want U", got.Colors)
		}
		if got.PT != "6/6" {
			t.Fatalf("Midnight Mangler P/T = %q, want 6/6", got.PT)
		}
	})
}

func farmPermanent(t *testing.T, snap OracleSnapshot, name string) OracleSnapPerm {
	t.Helper()
	for _, p := range snap.Permanents {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("%s is not on the battlefield in the %q snapshot: %+v", name, snap.Checkpoint, snap.Permanents)
	return OracleSnapPerm{}
}

func farmPermanentOnBoard(snap OracleSnapshot, name string) bool {
	for _, p := range snap.Permanents {
		if p.Name == name {
			return true
		}
	}
	return false
}
