package neutral_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// TestRebuildSelectObjectAndArrange covers the reverse translations the twin
// used to leave opaque: a select_object(single) with the protocol's "other"
// purpose (a named card or player), and a Dig's arrange_card / order_pick
// sequence with a filtered ChangeValid$ and a random remainder. Each pick must
// reach the neutral decision (NeutralOpaque false), not the agent's candidate-0
// forced answer.
func TestRebuildSelectObjectAndArrange(t *testing.T) {
	t.Run("select_object object single", func(t *testing.T) {
		tb := newTable(t, "Forest")
		tb.hand("Forest")
		h := tb.obs.Players[0].Hand[0]
		ref, err := json.Marshal(h.ObjectRef)
		if err != nil {
			t.Fatal(err)
		}
		cand := fmt.Sprintf(`{"kind":"select_object","source":null,"purpose":"other","choice":{"object":%s},"selected_count":0,"minimum":1,"maximum":1}`, ref)
		got := tb.ask(choiceCtx("null", "other"), [2]int{0, 1}, []string{cand})
		if tb.a.NeutralOpaque(0) {
			t.Fatalf("select_object(single) was not rebuilt: %s", tb.a.NeutralRebuild(0))
		}
		if id := objectID(got["choice"]); id != h.ObjectID {
			t.Fatalf("chose %q, want %q", id, h.ObjectID)
		}
	})

	t.Run("select_object player single", func(t *testing.T) {
		tb := newTable(t, "Forest")
		cand := `{"kind":"select_object","source":null,"purpose":"other","choice":{"player":"p1"},"selected_count":0,"minimum":1,"maximum":1}`
		tb.ask(choiceCtx("null", "other"), [2]int{0, 1}, []string{cand})
		if tb.a.NeutralOpaque(0) {
			t.Fatalf("player select_object(single) was not rebuilt: %s", tb.a.NeutralRebuild(0))
		}
	})

	t.Run("dig arrange_card and order_pick single", func(t *testing.T) {
		tb := newTable(t, "Thundertrap Trainer", "Island", "Glimpse the Cosmos", "Revenge of the Drowned", "Trip Up")
		srcRef := objRef("o-tt", "Thundertrap Trainer", "stack")
		tb.obs.Stack = []protocol.StackEntry{{ObjectRef: srcRef, StackKind: "triggered_ability"}}
		src, err := json.Marshal(srcRef)
		if err != nil {
			t.Fatal(err)
		}
		window := []string{"Island", "Glimpse the Cosmos", "Revenge of the Drowned", "Trip Up"}
		refs := tb.look("looked_at", window...)
		// Every card but the Island matches ChangeValid$ Card.nonCreature+nonLand.
		eligible := []bool{false, true, true, true}
		dests := make([]string, len(window))
		steps := 2*len(window) - 1
		for i := range window {
			var cands []string
			for _, d := range []string{"hand", "bottom"} {
				if d == "hand" && !eligible[i] {
					continue
				}
				cands = append(cands, fmt.Sprintf(`{"kind":"arrange_card","source":%s,"purpose":"dig","card":%s,"card_index":%d,"card_count":%d,"destination":%q}`,
					src, refs[i], i, len(window), d))
			}
			got := tb.ask(choiceCtx(string(src), "dig"), [2]int{i, steps}, cands)
			if tb.a.NeutralOpaque(0) {
				t.Fatalf("arrange_card %d was not rebuilt: %s", i, tb.a.NeutralRebuild(0))
			}
			dests[i] = got["destination"].(string)
		}
		// The Island stays on the bottom: RestRandomOrder poses one
		// presentational order_pick per remaining card, not a native order ask.
		i := slices.Index(dests, "bottom")
		if i < 0 {
			t.Fatalf("no card reached the bottom: %v", dests)
		}
		cand := fmt.Sprintf(`{"kind":"order_pick","source":%s,"purpose":"arrangement","item":{"object":%s},"position":0,"count":%d}`,
			src, refs[i], len(window))
		tb.ask(choiceCtx(string(src), "arrangement"), [2]int{len(window), steps}, []string{cand})
		if tb.a.NeutralOpaque(0) {
			t.Fatalf("order_pick(single) was not rebuilt: %s", tb.a.NeutralRebuild(0))
		}
	})
}
