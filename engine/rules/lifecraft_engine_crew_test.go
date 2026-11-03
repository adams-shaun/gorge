package rules

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Lifecraft Engine makes "Vehicle creatures you control" the chosen type, and
// crewing is what makes the Engine a creature. Both are layer 4 effects, and
// the Engine's applies to a set (CR 613.8a) that the crew effect changes, so
// the crew effect applies first and the crewed Engine is of the chosen type
// whichever effect has the later timestamp.
func TestLifecraftEngineCrewedIsTheChosenType(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := map[string]any{
		"name": t.Name(), "turn": 1, "why": "",
		"setup": map[string]any{
			"p0": map[string]any{
				"battlefield": []string{"Mountain", "Dynamite Diver", "Island"},
				"hand":        []string{"Lifecraft Engine", "Island", "Island"},
			},
			"p1": map[string]any{"battlefield": []string{"Island"}},
		},
		"steps": []map[string]any{
			{"op": "play", "seat": 0, "card": "p0:Island#2"},
			{"op": "activate", "seat": 0, "card": "p0:Mountain", "ability_index": 0},
			{"op": "activate", "seat": 0, "card": "p0:Island", "ability_index": 0},
			{"op": "activate", "seat": 0, "card": "p0:Island#2", "ability_index": 0},
			{"op": "cast", "seat": 0, "card": "p0:Lifecraft Engine"},
			{"op": "pass", "seat": 0},
			{"op": "pass", "seat": 1},
			{"op": "activate", "seat": 0, "card": "p0:Lifecraft Engine", "ability_index": 0,
				"answers": []map[string]any{
					{"kind": "choose", "pick": []string{"Artificer"}},
					{"kind": "choose", "pick": []string{"p0:Dynamite Diver"}},
				}},
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
		t.Fatal(err)
	}
	if len(res.Fails) > 0 {
		t.Fatalf("scenario failed: %v", res.Fails)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	for _, p := range last.Permanents {
		if p.Name != "Lifecraft Engine" {
			continue
		}
		if !slices.Contains(p.Types, "Creature") || !slices.Contains(p.Types, "Artificer") {
			t.Fatalf("crewed Lifecraft Engine types = %v, want a creature of the chosen type Artificer", p.Types)
		}
		return
	}
	t.Fatal("Lifecraft Engine is not on the battlefield")
}
