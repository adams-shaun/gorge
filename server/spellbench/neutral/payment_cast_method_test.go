//go:build spellbench

package neutral_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// TestNeutralPaymentKeepsCastMethod: a manual host (no engine_autopay) poses
// Gollum's adventure and creature casts as separate candidates. Mountain,
// Island and Plains are already tapped, so the pool floats {R}{U}{W} and the
// Swamp is the one source left: gorge's planner offers a plan for each cast
// and the bot (which keeps the first plan of a card, the adventure's) picks one.
// The lowering taps the Swamp, and at the cast window both methods are offered
// again. The plan must not shadow the adventure cast there: the bot picks the
// adventure (the tie-break its native decision makes) and the answer is the
// candidate whose method is "adventure", never the plain cast.
func TestNeutralPaymentKeepsCastMethod(t *testing.T) {
	reg := testcorpus.Registry(t)
	b := payBoard{lands: []string{"Mountain", "Island", "Plains", "Swamp"}, hand: []string{"Gollum, Silent Slinker"},
		tapped: map[string]bool{"Mountain": true, "Island": true, "Plains": true}}
	// The Swamp is the only untapped source, so whichever plan the bot takes
	// is the lowering of a tap of it; both casts are then payable.
	plan := []string{"Swamp"}
	candidates := func() []string {
		out := []string{`{"kind":"pass"}`}
		for _, l := range b.lands {
			if !b.tapped[l] {
				out = append(out, `{"kind":"activate_mana_ability","source":`+ref0("battlefield", l)+`,"ability_index":0,"mana_choice":null,"cost_target":null}`)
			}
		}
		total := 0
		for _, n := range b.pool() {
			total += n
		}
		if b.pool()["B"] >= 1 {
			out = append(out, `{"kind":"cast_spell","source":`+ref0("hand", "Gollum, Silent Slinker")+`,"method":"adventure"}`)
		}
		if total >= 4 && b.pool()["B"] >= 1 {
			out = append(out, `{"kind":"cast_spell","source":`+ref0("hand", "Gollum, Silent Slinker")+`,"method":"normal"}`)
		}
		return out
	}
	deck := `{"name":"Mix","decklist":[{"name":"Gollum, Silent Slinker","count":4},{"name":"Mountain","count":4},{"name":"Island","count":4},{"name":"Plains","count":4},{"name":"Swamp","count":4}]}`
	a := neutralAgent(t, reg, deck)
	var taps []string
	for step := 2; step < 10; step++ {
		cs := candidates()
		i := pick(t, a, chooseLine(step, b, cs))
		var sem protocol.Semantic
		if err := json.Unmarshal([]byte(cs[i]), &sem); err != nil {
			t.Fatal(err)
		}
		switch sem.Kind {
		case "activate_mana_ability":
			var src struct {
				Source struct {
					CardName string `json:"card_name"`
				} `json:"source"`
			}
			if err := json.Unmarshal([]byte(cs[i]), &src); err != nil {
				t.Fatal(err)
			}
			taps = append(taps, src.Source.CardName)
			b.tapped[src.Source.CardName] = true
		case "cast_spell":
			// Precondition: the lowering took gorge's whole plan for the
			// creature cast, so a plan (not the manual tap gate) chose these
			// sources.
			if !slices.Equal(taps, plan) {
				t.Fatalf("the lowering tapped %v, gorge's plan is %v", taps, plan)
			}
			// Precondition: both methods are on offer, and they differ.
			var methods []string
			for _, c := range cs {
				var other protocol.Semantic
				json.Unmarshal([]byte(c), &other)
				if other.Kind == "cast_spell" {
					methods = append(methods, str0(other, "method"))
				}
			}
			if !slices.Equal(methods, []string{"adventure", "normal"}) {
				t.Fatalf("the cast window offers methods %v, want the adventure beside the creature cast", methods)
			}
			if got := str0(sem, "method"); got != "adventure" {
				t.Fatalf("the seat cast %q; the plan for the plain cast must not shadow the adventure", got)
			}
			if a.Fallbacks() != 0 {
				t.Fatalf("%d fallbacks; the bot's own choice was not mapped", a.Fallbacks())
			}
			return
		default:
			t.Fatalf("the seat answered %s at step %d", sem.Kind, step)
		}
	}
	t.Fatal("the seat never reached the cast")
}

func str0(sem protocol.Semantic, k string) string {
	if v, ok := sem.Fields[k].(string); ok {
		return v
	}
	return ""
}
