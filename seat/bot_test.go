package seat

import (
	"context"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// TestBotIsDeterministic is Ruling P8's whole point: the same bot seed
// answering the same decision sequence produces identical intents, so a
// match is reproducible from (engine seed, bot seed) alone. The sequence
// below deliberately includes the two rng-consuming kinds (KBlockers,
// KTriggerOrder) as well as a coin-flip one (KTriggerOptional), since a bot
// whose rng leaked from the engine or the process clock would only show it
// on paths that actually draw from it.
func TestBotIsDeterministic(t *testing.T) {
	seq := []decision.Decision{
		{Seq: 1, Player: 0, Kind: decision.KBlockers, Min: 0, Max: 3, Options: []decision.Option{
			{Index: 0, Kind: "block", Obj: 10, Attacker: 20},
			{Index: 1, Kind: "block", Obj: 10, Attacker: 21},
			{Index: 2, Kind: "block", Obj: 11, Attacker: 20},
		}},
		{Seq: 2, Player: 0, Kind: decision.KTriggerOrder, Min: 3, Max: 3, Options: []decision.Option{
			{Index: 0, Kind: "trigger", Obj: 30},
			{Index: 1, Kind: "trigger", Obj: 31},
			{Index: 2, Kind: "trigger", Obj: 32},
		}},
		{Seq: 3, Player: 0, Kind: decision.KTriggerOptional, Min: 1, Max: 1, Options: []decision.Option{
			{Index: 0, Kind: "yes"}, {Index: 1, Kind: "no"},
		}},
	}
	run := func(seed uint64) []decision.Intent {
		b := NewBot(seed)
		out := make([]decision.Intent, len(seq))
		for i, d := range seq {
			in, err := b.Decide(context.Background(), view.View{}, d)
			if err != nil {
				t.Fatalf("Decide returned an error: %v", err)
			}
			out[i] = in
		}
		return out
	}
	a, b := run(7), run(7)
	for i := range seq {
		if len(a[i].Choices) != len(b[i].Choices) {
			t.Fatalf("decision %d: choice lengths differ between runs: %v vs %v", i, a[i], b[i])
		}
		for j := range a[i].Choices {
			if a[i].Choices[j] != b[i].Choices[j] {
				t.Fatalf("decision %d: choices differ between runs: %v vs %v", i, a[i], b[i])
			}
		}
	}
}

// TestBotOnlyChoosesOfferedOptions is Ruling P7's completeness requirement
// made concrete: every decision.Kind decision.go defines (eight today --
// the brief and supplement both say seven, predating KModes) gets a
// representative Decision here, including the two edge shapes the fuzz gate
// depends on (Min == Max == N for KTriggerOrder, Min == Max == 1 over two
// options for KTriggerOptional) and an empty option list with Min == 0 (the
// generic fallback's own edge case). Every resulting intent must validate
// against the decision it answered -- an intent Validate rejects is a bot
// bug, full stop.
func TestBotOnlyChoosesOfferedOptions(t *testing.T) {
	cases := []struct {
		name string
		d    decision.Decision
	}{
		{"priority", decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
			Options: []decision.Option{
				{Index: 0, Kind: "activate", Obj: 1},
				{Index: 1, Kind: "play_land", Obj: 2},
				{Index: 2, Kind: "cast", Obj: 3},
				{Index: 3, Kind: "pass"},
			}}},
		{"target", decision.Decision{Seq: 2, Player: 0, Kind: decision.KTarget, Min: 1, Max: 1,
			Options: []decision.Option{
				{Index: 0, Kind: "player", Player: 0},
				{Index: 1, Kind: "player", Player: 1},
				{Index: 2, Kind: "permanent", Obj: 10, Player: 1},
			}}},
		{"attackers", decision.Decision{Seq: 3, Player: 0, Kind: decision.KAttackers, Min: 0, Max: 2,
			Options: []decision.Option{
				{Index: 0, Kind: "attacker", Obj: 20, Player: 1},
				{Index: 1, Kind: "attacker", Obj: 21, Player: 1},
			}}},
		{"blockers", decision.Decision{Seq: 4, Player: 1, Kind: decision.KBlockers, Min: 0, Max: 3,
			Options: []decision.Option{
				{Index: 0, Kind: "block", Obj: 30, Attacker: 20, Player: 1},
				{Index: 1, Kind: "block", Obj: 30, Attacker: 21, Player: 1},
				{Index: 2, Kind: "block", Obj: 31, Attacker: 20, Player: 1},
			}}},
		{"mulligan", decision.Decision{Seq: 5, Player: 0, Kind: decision.KMulligan, Min: 1, Max: 1,
			Options: []decision.Option{
				{Index: 0, Kind: "keep"},
				{Index: 1, Kind: "mulligan"},
			}}},
		{"modes-empty", decision.Decision{Seq: 6, Player: 0, Kind: decision.KModes, Min: 0, Max: 0}},
		{"trigger_order", decision.Decision{Seq: 7, Player: 0, Kind: decision.KTriggerOrder, Min: 3, Max: 3,
			Options: []decision.Option{
				{Index: 0, Kind: "trigger", Obj: 40},
				{Index: 1, Kind: "trigger", Obj: 41},
				{Index: 2, Kind: "trigger", Obj: 42},
			}}},
		{"trigger_optional", decision.Decision{Seq: 8, Player: 0, Kind: decision.KTriggerOptional, Min: 1, Max: 1,
			Options: []decision.Option{
				{Index: 0, Kind: "yes", Obj: 50},
				{Index: 1, Kind: "no", Obj: 50},
			}}},
	}

	seenKinds := map[decision.Kind]bool{}
	b := NewBot(99)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seenKinds[tc.d.Kind] = true
			in, err := b.Decide(context.Background(), view.View{}, tc.d)
			if err != nil {
				t.Fatalf("Decide returned an error: %v", err)
			}
			if err := tc.d.Validate(in); err != nil {
				t.Fatalf("bot's own intent failed Validate: %v (intent=%+v)", err, in)
			}
		})
	}
	allKinds := []decision.Kind{decision.KPriority, decision.KTarget, decision.KAttackers,
		decision.KBlockers, decision.KMulligan, decision.KModes, decision.KTriggerOrder,
		decision.KTriggerOptional}
	for _, k := range allKinds {
		if !seenKinds[k] {
			t.Errorf("decision.Kind %q was never exercised by this test", k)
		}
	}
}

// TestBotAlwaysAnswersEveryDecisionKind checks the brief's four headline
// kinds behave like the aggro policy the doc comment promises, not merely
// that they validate: priority prefers tapping mana over a land drop or a
// cast, target prefers an opponent, attackers commits every legal attacker,
// and blockers never assigns one blocker to two attackers.
func TestBotAlwaysAnswersEveryDecisionKind(t *testing.T) {
	b := NewBot(1)
	ctx := context.Background()

	priority := decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "play_land", Obj: 1},
			{Index: 1, Kind: "activate", Obj: 2},
			{Index: 2, Kind: "cast", Obj: 3},
			{Index: 3, Kind: "pass"},
		}}
	in, err := b.Decide(ctx, view.View{}, priority)
	if err != nil || len(in.Choices) != 1 || priority.Options[in.Choices[0]].Kind != "activate" {
		t.Errorf("priority = %+v (err=%v), want the lone activate option chosen first", in, err)
	}

	target := decision.Decision{Seq: 2, Player: 0, Kind: decision.KTarget, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "player", Player: 0},
			{Index: 1, Kind: "player", Player: 1},
		}}
	in, err = b.Decide(ctx, view.View{}, target)
	if err != nil || len(in.Choices) != 1 || target.Options[in.Choices[0]].Player != 1 {
		t.Errorf("target = %+v (err=%v), want the opposing player chosen", in, err)
	}

	attackers := decision.Decision{Seq: 3, Player: 0, Kind: decision.KAttackers, Min: 0, Max: 2,
		Options: []decision.Option{
			{Index: 0, Kind: "attacker", Obj: 10},
			{Index: 1, Kind: "attacker", Obj: 11},
		}}
	in, err = b.Decide(ctx, view.View{}, attackers)
	if err != nil || len(in.Choices) != 2 {
		t.Errorf("attackers = %+v (err=%v), want every legal attacker chosen", in, err)
	}

	blockers := decision.Decision{Seq: 4, Player: 1, Kind: decision.KBlockers, Min: 0, Max: 2,
		Options: []decision.Option{
			{Index: 0, Kind: "block", Obj: 20, Attacker: 10},
			{Index: 1, Kind: "block", Obj: 20, Attacker: 11}, // same blocker, second attacker
		}}
	in, err = b.Decide(ctx, view.View{}, blockers)
	if err != nil {
		t.Fatalf("blockers returned an error: %v", err)
	}
	if err := blockers.Validate(in); err != nil {
		t.Fatalf("blockers intent failed Validate: %v", err)
	}
	used := map[state.ObjID]bool{}
	for _, c := range in.Choices {
		obj := blockers.Options[c].Obj
		if used[obj] {
			t.Errorf("blockers = %+v assigned blocker %d to more than one attacker", in, obj)
		}
		used[obj] = true
	}
}
