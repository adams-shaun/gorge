package rules

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// firstOfKindStack plays a scenario and returns how many abilities are on the
// stack after each cast of a card named in casts, in order.
func firstOfKindStack(t *testing.T, setup map[string]any, seat int, casts ...string) []int {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	steps := []map[string]any{}
	for _, c := range casts {
		steps = append(steps,
			map[string]any{"op": "cast", "seat": seat, "card": c, "mana": "UUUGG"},
			map[string]any{"op": "resolve", "seat": seat},
		)
	}
	raw, err := json.Marshal(map[string]any{"name": t.Name(), "turn": 1, "why": "", "setup": setup, "steps": steps})
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
	var out []int
	for _, s := range res.Snapshots {
		if !strings.HasSuffix(s.Checkpoint, "(cast)") {
			continue
		}
		abilities := 0
		for _, e := range s.Stack {
			if e.Kind == "ability" {
				abilities++
			}
		}
		out = append(out, abilities)
	}
	return out
}

// ActivatorThisTurnCast$ counts the spells that match ValidCard$, not every
// spell: Forge's TriggerSpellAbilityCastOrCopy reads its comparison through
// CardUtil.getThisTurnCast with the trigger's ValidCard$, and XMage's "first
// noncreature spell" watchers count only that kind.
func TestActivatorThisTurnCastCountsValidCardSpells(t *testing.T) {
	t.Parallel()
	p1 := map[string]any{"battlefield": []string{"Island"}}
	p0 := func(hand ...string) map[string]any {
		return map[string]any{
			"p0": map[string]any{
				"battlefield": []string{"Aquatic Alchemist"},
				"hand":        hand,
			},
			"p1": p1,
		}
	}
	for _, tc := range []struct {
		name  string
		hand  []string
		casts []string
		want  []int
	}{
		{
			name:  "first sorcery after a creature spell fires",
			hand:  []string{"Grizzly Bears", "Divination"},
			casts: []string{"p0:Grizzly Bears", "p0:Divination"},
			want:  []int{0, 1},
		},
		{
			name:  "second sorcery does not",
			hand:  []string{"Divination", "Divination"},
			casts: []string{"p0:Divination", "p0:Divination#2"},
			want:  []int{1, 0},
		},
		{
			name:  "a spell ValidCard turns away does not",
			hand:  []string{"Grizzly Bears", "Grizzly Bears"},
			casts: []string{"p0:Grizzly Bears", "p0:Grizzly Bears#2"},
			want:  []int{0, 0},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := firstOfKindStack(t, p0(tc.hand...), 0, tc.casts...)
			if len(got) != len(tc.want) {
				t.Fatalf("casts seen %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("abilities on the stack after each cast %v, want %v", got, tc.want)
				}
			}
		})
	}
}
