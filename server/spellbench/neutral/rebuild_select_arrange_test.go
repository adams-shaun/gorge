package neutral_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// requireTranslated fails when any candidate of the last neutral decision was
// left untranslated: the twin excludes such a decision instead of comparing it.
func requireTranslated(t *testing.T, tb *table, n int, what string) {
	t.Helper()
	for i := 0; i < n; i++ {
		if tb.a.NeutralOpaque(i) {
			t.Fatalf("%s candidate %d was not rebuilt: %s", what, i, tb.a.NeutralRebuild(i))
		}
	}
}

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
		requireTranslated(t, tb, 1, "select_object(single)")
		if id := objectID(got["choice"]); id != h.ObjectID {
			t.Fatalf("chose %q, want %q", id, h.ObjectID)
		}
	})

	t.Run("select_object player single", func(t *testing.T) {
		tb := newTable(t, "Forest")
		cand := `{"kind":"select_object","source":null,"purpose":"other","choice":{"player":"p1"},"selected_count":0,"minimum":1,"maximum":1}`
		tb.ask(choiceCtx("null", "other"), [2]int{0, 1}, []string{cand})
		requireTranslated(t, tb, 1, "player select_object(single)")
	})

	t.Run("select_object proliferate", func(t *testing.T) {
		tb := newTable(t, "Contentious Plan", "Island")
		src := tb.spell("o-cp", "Contentious Plan")
		a := objRef("o-pa", "Island", "battlefield")
		b := objRef("o-pb", "Forest", "battlefield")
		for _, r := range []protocol.ObjectRef{a, b} {
			r := r
			tb.obs.Players[0].Battlefield = append(tb.obs.Players[0].Battlefield, protocol.ObjectRecord{ObjectRef: r,
				Characteristics: &protocol.Characteristics{Types: []string{"land"}, Supertypes: []string{}, Subtypes: []string{}, Colors: []string{}, Keywords: []string{}}})
		}
		ra, _ := json.Marshal(a)
		rb, _ := json.Marshal(b)
		cands := []string{
			fmt.Sprintf(`{"kind":"select_object","source":%s,"purpose":"other","choice":{"object":%s},"selected_count":0,"minimum":0,"maximum":2}`, src, ra),
			fmt.Sprintf(`{"kind":"select_object","source":%s,"purpose":"other","choice":{"object":%s},"selected_count":0,"minimum":0,"maximum":2}`, src, rb),
			fmt.Sprintf(`{"kind":"finish_selection","source":%s,"purpose":"other","selected_count":0}`, src),
		}
		got := tb.ask(choiceCtx(src, "other"), [2]int{0, 3}, cands)
		requireTranslated(t, tb, len(cands), "proliferate select_object")
		// The proliferate arm takes the first Max, so the first pick is an
		// object, not the finish option a generic first-Min fill would take.
		if got["kind"] != "select_object" {
			t.Fatalf("proliferate pick chose %v, want the first offered object", got["kind"])
		}
		// The second substep proves the plan kept the remaining Max choices:
		// a first-Min plan has spent its only pick and would decline here.
		cands = []string{
			fmt.Sprintf(`{"kind":"select_object","source":%s,"purpose":"other","choice":{"object":%s},"selected_count":1,"minimum":0,"maximum":2}`, src, rb),
			fmt.Sprintf(`{"kind":"finish_selection","source":%s,"purpose":"other","selected_count":1}`, src),
		}
		got = tb.ask(choiceCtx(src, "other"), [2]int{1, 3}, cands)
		requireTranslated(t, tb, len(cands), "proliferate select_object second")
		if got["kind"] != "select_object" {
			t.Fatalf("proliferate second pick chose %v, want the remaining offered object", got["kind"])
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
			requireTranslated(t, tb, len(cands), fmt.Sprintf("arrange_card %d", i))
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
		requireTranslated(t, tb, 1, "order_pick(single)")
	})

	// A hand-bound order pick is the native Choices order, not a remainder's
	// presentational dest: the plan's first taken card wins even when it is
	// not candidate 0. Lead the Stampede takes every creature to hand.
	t.Run("order_pick on the taken hand keeps the Choices order", func(t *testing.T) {
		tb := newTable(t, "Lead the Stampede", "Wall of Roots", "Lotus Petal", "Masked Vandal", "Forest")
		srcRef := objRef("o-lts", "Lead the Stampede", "stack")
		tb.obs.Stack = []protocol.StackEntry{{ObjectRef: srcRef, StackKind: "spell"}}
		src, err := json.Marshal(srcRef)
		if err != nil {
			t.Fatal(err)
		}
		window := []string{"Wall of Roots", "Lotus Petal", "Masked Vandal", "Forest", "Island"}
		refs := tb.look("looked_at", window...)
		creature := []bool{true, false, true, false, false}
		steps := 2*len(window) - 1
		var hand []int
		for i := range window {
			var cands []string
			for _, d := range []string{"bottom", "hand"} {
				if d == "hand" && !creature[i] {
					continue
				}
				cands = append(cands, fmt.Sprintf(`{"kind":"arrange_card","source":%s,"purpose":"dig","card":%s,"card_index":%d,"card_count":%d,"destination":%q}`,
					src, refs[i], i, len(window), d))
			}
			got := tb.ask(choiceCtx(string(src), "dig"), [2]int{i, steps}, cands)
			requireTranslated(t, tb, len(cands), fmt.Sprintf("arrange_card %d", i))
			if got["destination"] == "hand" {
				hand = append(hand, i)
			}
		}
		if len(hand) < 2 {
			t.Fatalf("fewer than two creatures reached hand: %v", hand)
		}
		// The native options follow hiddenOrder (by card name), so the plan's
		// first hand pick is the name-first taken card.
		slices.SortStableFunc(hand, func(x, y int) int { return strings.Compare(window[x], window[y]) })
		// Offer the plan's second choice first: an always-matching dest op
		// would take it, the Choices list op picks the name-first card.
		var cands []string
		for _, i := range []int{hand[1], hand[0]} {
			cands = append(cands, fmt.Sprintf(`{"kind":"order_pick","source":%s,"purpose":"arrangement","item":{"object":%s},"position":0,"count":%d}`,
				src, refs[i], len(window)))
		}
		got := objectID(tb.ask(choiceCtx(string(src), "arrangement"), [2]int{len(window), steps}, cands)["item"])
		requireTranslated(t, tb, len(cands), "order_pick(hand)")
		if want := fmt.Sprintf("o-l%d", hand[0]); got != want {
			t.Fatalf("hand order picked %s first, want the plan's first choice %s", got, want)
		}
	})
}
