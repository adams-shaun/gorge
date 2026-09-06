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
//
// M1 (fix round 1): run(7) is also compared against run(8) -- comparing
// run(7) to itself alone cannot tell "seeded" from "constant"; deleting the
// bot's rng entirely (a fixed choice in KBlockers/KTriggerOrder/
// KTriggerOptional) would still pass a same-seed-only check.
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
	same := func(a, b decision.Intent) bool {
		if len(a.Choices) != len(b.Choices) {
			return false
		}
		for i := range a.Choices {
			if a.Choices[i] != b.Choices[i] {
				return false
			}
		}
		return true
	}

	a, b := run(7), run(7)
	for i := range seq {
		if !same(a[i], b[i]) {
			t.Fatalf("decision %d: choices differ between two runs of the same seed: %v vs %v", i, a[i], b[i])
		}
	}

	c := run(8)
	allSame := true
	for i := range seq {
		if !same(a[i], c[i]) {
			allSame = false
		}
	}
	if allSame {
		t.Fatal("seeds 7 and 8 produced identical intents on every decision -- the rng may not be load-bearing")
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
// bug, full stop. The priority case is answered in a main phase, so the
// activate branch (Ruling T25-b, fix round 1's isMain gate) gets exercised
// here too, not only in TestBotAlwaysAnswersEveryDecisionKind.
func TestBotOnlyChoosesOfferedOptions(t *testing.T) {
	cases := []struct {
		name string
		d    decision.Decision
		v    view.View
	}{
		{"priority", decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
			Options: []decision.Option{
				{Index: 0, Kind: "activate", Obj: 1},
				{Index: 1, Kind: "play_land", Obj: 2},
				{Index: 2, Kind: "cast", Obj: 3},
				{Index: 3, Kind: "pass"},
			}}, view.View{Phase: "main1"}},
		{"target", decision.Decision{Seq: 2, Player: 0, Kind: decision.KTarget, Min: 1, Max: 1,
			Options: []decision.Option{
				{Index: 0, Kind: "player", Player: 0},
				{Index: 1, Kind: "player", Player: 1},
				{Index: 2, Kind: "permanent", Obj: 10, Player: 1},
			}}, view.View{}},
		{"attackers", decision.Decision{Seq: 3, Player: 0, Kind: decision.KAttackers, Min: 0, Max: 2,
			Options: []decision.Option{
				{Index: 0, Kind: "attacker", Obj: 20, Player: 1},
				{Index: 1, Kind: "attacker", Obj: 21, Player: 1},
			}}, view.View{}},
		{"blockers", decision.Decision{Seq: 4, Player: 1, Kind: decision.KBlockers, Min: 0, Max: 3,
			Options: []decision.Option{
				{Index: 0, Kind: "block", Obj: 30, Attacker: 20, Player: 1},
				{Index: 1, Kind: "block", Obj: 30, Attacker: 21, Player: 1},
				{Index: 2, Kind: "block", Obj: 31, Attacker: 20, Player: 1},
			}}, view.View{}},
		{"mulligan", decision.Decision{Seq: 5, Player: 0, Kind: decision.KMulligan, Min: 1, Max: 1,
			Options: []decision.Option{
				{Index: 0, Kind: "keep"},
				{Index: 1, Kind: "mulligan"},
			}}, view.View{}},
		{"modes-empty", decision.Decision{Seq: 6, Player: 0, Kind: decision.KModes, Min: 0, Max: 0}, view.View{}},
		{"trigger_order", decision.Decision{Seq: 7, Player: 0, Kind: decision.KTriggerOrder, Min: 3, Max: 3,
			Options: []decision.Option{
				{Index: 0, Kind: "trigger", Obj: 40},
				{Index: 1, Kind: "trigger", Obj: 41},
				{Index: 2, Kind: "trigger", Obj: 42},
			}}, view.View{}},
		{"trigger_optional", decision.Decision{Seq: 8, Player: 0, Kind: decision.KTriggerOptional, Min: 1, Max: 1,
			Options: []decision.Option{
				{Index: 0, Kind: "yes", Obj: 50},
				{Index: 1, Kind: "no", Obj: 50},
			}}, view.View{}},
	}

	seenKinds := map[decision.Kind]bool{}
	b := NewBot(99)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seenKinds[tc.d.Kind] = true
			in, err := b.Decide(context.Background(), tc.v, tc.d)
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

// TestBotTotalityUnderArbitraryMinMax is I-4/Ruling T25-c's regression test:
// every kind-specific branch used to ignore d.Min/d.Max entirely, so it only
// validated by coincidence of the shapes today's engine happens to emit.
// 10,000 random decisions (random kind among all eight, 0-8 options with
// random Kind/Obj/Player fields, random 0 <= Min <= Max <= len(Options)) must
// all validate and never panic.
func TestBotTotalityUnderArbitraryMinMax(t *testing.T) {
	kinds := []decision.Kind{decision.KPriority, decision.KTarget, decision.KAttackers,
		decision.KBlockers, decision.KMulligan, decision.KModes, decision.KTriggerOrder,
		decision.KTriggerOptional}
	optKinds := []string{"activate", "play_land", "cast", "pass", "concede", "player", "permanent",
		"attacker", "block", "trigger", "yes", "no", "keep", "mulligan", "whatever"}

	// A small, dependency-free xorshift so this test needs no package-level
	// rng and stays reproducible on its own: seeded once, consumed
	// sequentially, never the bot's own rng and never a global.
	var s uint64 = 0x2545F4914F6CDD1D
	next := func(n int) int {
		s ^= s << 13
		s ^= s >> 7
		s ^= s << 17
		if n <= 0 {
			return 0
		}
		return int(s % uint64(n))
	}

	b := NewBot(42)
	for i := 0; i < 10000; i++ {
		nOpts := next(9) // 0..8
		opts := make([]decision.Option, nOpts)
		for j := range opts {
			opts[j] = decision.Option{
				Index:  j,
				Kind:   optKinds[next(len(optKinds))],
				Obj:    state.ObjID(next(5)),
				Player: state.PlayerID(next(4)),
			}
		}
		min := next(nOpts + 1)
		max := min + next(nOpts-min+1)
		d := decision.Decision{Seq: uint64(i), Player: state.PlayerID(next(4)),
			Kind: kinds[next(len(kinds))], Min: min, Max: max, Options: opts}

		var in decision.Intent
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("case %d: Decide panicked on %+v: %v", i, d, r)
				}
			}()
			in, err = b.Decide(context.Background(), view.View{}, d)
		}()
		if err != nil {
			t.Fatalf("case %d: Decide returned an error on %+v: %v", i, d, err)
		}
		if verr := d.Validate(in); verr != nil {
			t.Fatalf("case %d: intent %+v failed Validate against %+v: %v", i, in, d, verr)
		}
	}
}

// TestBotPassesOutsideMainWithNoCastOrLandDrop is Ruling T25-g's regression
// test, using the engine's REAL shape for a non-main priority decision
// (rules/legal.go's legalActions never offers play_land or cast outside
// sorcery speed/an affordable instant): one or more "activate" options
// followed by "pass" last. Fix round 1's own regression case for this
// (TestBotAlwaysAnswersEveryDecisionKind's "priority (combat)" case, above)
// included a play_land option that the switch's un-gated second loop
// matches regardless of phase, so it never actually reached the fallback
// path that reintroduced I-1(b) in a live game (clamp topping up a
// Min:1/Max:1 decision with the first unused option, which legalActions
// always lists as an activation before pass). This decision has no
// play_land and no cast, so the bot must fall through to the explicit pass
// added in botDecide's KPriority case -- not clamp's top-up -- for every
// phase that isn't a main phase.
func TestBotPassesOutsideMainWithNoCastOrLandDrop(t *testing.T) {
	d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "activate", Obj: 100},
			{Index: 1, Kind: "activate", Obj: 101},
			{Index: 2, Kind: "pass"},
			{Index: 3, Kind: "concede"},
		}}
	for _, phase := range []string{"", "beginning", "combat", "ending"} {
		in, err := NewBot(1).Decide(context.Background(), view.View{Phase: phase}, d)
		if err != nil {
			t.Fatalf("phase %q: Decide returned an error: %v", phase, err)
		}
		if err := d.Validate(in); err != nil {
			t.Fatalf("phase %q: intent %+v failed Validate: %v", phase, in, err)
		}
		if len(in.Choices) != 1 || d.Options[in.Choices[0]].Kind != "pass" {
			t.Errorf("phase %q: priority = %+v, want pass chosen -- not an activation -- outside a main phase", phase, in)
		}
	}
	// And inside a main phase, the same decision DOES prefer activate --
	// confirming this test isn't just checking "never activate ever".
	in, err := NewBot(1).Decide(context.Background(), view.View{Phase: "main1"}, d)
	if err != nil || len(in.Choices) != 1 || d.Options[in.Choices[0]].Kind != "activate" {
		t.Errorf("phase \"main1\": priority = %+v (err=%v), want an activation chosen", in, err)
	}
	// M2d-3: the trailing "concede" option is never chosen in either phase
	// -- the whole reason it sits after "pass", where only an explicit
	// pick (or a blind default-to-final client) could reach it.
	for _, phase := range []string{"", "beginning", "combat", "ending", "main1", "main2"} {
		in, err := NewBot(1).Decide(context.Background(), view.View{Phase: phase}, d)
		if err != nil || len(in.Choices) == 0 || d.Options[in.Choices[0]].Kind == "concede" {
			t.Errorf("phase %q: bot chose concede or errored: %+v (err=%v)", phase, in, err)
		}
	}
}

func TestBotChoosePolicy(t *testing.T) {
	b := NewBot(1)
	choose := func(kind string, n, min, max int) decision.Intent {
		d := decision.Decision{Player: 0, Kind: decision.KChoose, Min: min, Max: max}
		for i := 0; i < n; i++ {
			d.Options = append(d.Options, decision.Option{Index: i, Kind: kind, Label: kind})
		}
		if kind == "yes" {
			d.Options = []decision.Option{{Index: 0, Kind: "yes"}, {Index: 1, Kind: "no"}}
		}
		in, _ := b.Decide(context.Background(), view.View{}, d)
		return in
	}
	if got := choose("x", 4, 1, 1).Choices; len(got) != 1 || got[0] != 3 {
		t.Fatalf("x: %v, want the highest", got)
	}
	if got := choose("exile", 5, 0, 3).Choices; len(got) != 3 || got[0] != 0 || got[2] != 2 {
		t.Fatalf("exile: %v, want the first three", got)
	}
	if got := choose("sacrifice", 2, 1, 1).Choices; len(got) != 1 || got[0] != 0 {
		t.Fatalf("sacrifice: %v", got)
	}
	if got := choose("yes", 2, 1, 1).Choices; len(got) != 1 || got[0] != 0 {
		t.Fatalf("yes/no: %v, want yes", got)
	}
	for _, k := range []string{"name", "type", "number"} {
		if got := choose(k, 3, 1, 1).Choices; len(got) != 1 || got[0] != 0 {
			t.Fatalf("%s: %v, want the first", k, got)
		}
	}
}

// TestBotAlwaysAnswersEveryDecisionKind checks the brief's four headline
// kinds behave like the aggro policy the doc comment promises, not merely
// that they validate: priority prefers tapping mana over a land drop or a
// cast (Ruling T25-b: only in a main phase, so the view passed here is one),
// target prefers an opponent, attackers commits every legal attacker, and
// blockers never assigns one blocker to two attackers.
func TestBotAlwaysAnswersEveryDecisionKind(t *testing.T) {
	ctx := context.Background()

	priority := decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "play_land", Obj: 1},
			{Index: 1, Kind: "activate", Obj: 2},
			{Index: 2, Kind: "cast", Obj: 3},
			{Index: 3, Kind: "pass"},
		}}
	in, err := NewBot(1).Decide(ctx, view.View{Phase: "main1"}, priority)
	if err != nil || len(in.Choices) != 1 || priority.Options[in.Choices[0]].Kind != "activate" {
		t.Errorf("priority (main phase) = %+v (err=%v), want the lone activate option chosen first", in, err)
	}
	// Outside a main phase the same options must NOT prefer activate --
	// this is the whole point of the fix: it would rather play its land.
	in, err = NewBot(1).Decide(ctx, view.View{Phase: "combat"}, priority)
	if err != nil || len(in.Choices) != 1 || priority.Options[in.Choices[0]].Kind == "activate" {
		t.Errorf("priority (combat) = %+v (err=%v), want activate NOT chosen outside a main phase", in, err)
	}

	target := decision.Decision{Seq: 2, Player: 0, Kind: decision.KTarget, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "player", Player: 0},
			{Index: 1, Kind: "player", Player: 1},
		}}
	in, err = NewBot(1).Decide(ctx, view.View{}, target)
	if err != nil || len(in.Choices) != 1 || target.Options[in.Choices[0]].Player != 1 {
		t.Errorf("target = %+v (err=%v), want the opposing player chosen", in, err)
	}

	attackers := decision.Decision{Seq: 3, Player: 0, Kind: decision.KAttackers, Min: 0, Max: 2,
		Options: []decision.Option{
			{Index: 0, Kind: "attacker", Obj: 10},
			{Index: 1, Kind: "attacker", Obj: 11},
		}}
	in, err = NewBot(1).Decide(ctx, view.View{}, attackers)
	if err != nil || len(in.Choices) != 2 {
		t.Errorf("attackers = %+v (err=%v), want every legal attacker chosen", in, err)
	}

	// M2 (fix round 1): a single fixed seed is fragile -- with NewBot(1) the
	// two coin flips below did not both land "block" even with the used map
	// deleted entirely, so the assertion passed for the wrong reason. Three
	// options share blocker 20 here, and every one of 50 seeds is checked:
	// with the used map removed, at least one seed assigns blocker 20 to two
	// (or three) attackers with overwhelming probability (1 - 0.5^50 that
	// SOME seed shows it), while the real policy must never do so for ANY
	// seed.
	blockers := decision.Decision{Seq: 4, Player: 1, Kind: decision.KBlockers, Min: 0, Max: 4,
		Options: []decision.Option{
			{Index: 0, Kind: "block", Obj: 20, Attacker: 10},
			{Index: 1, Kind: "block", Obj: 20, Attacker: 11},
			{Index: 2, Kind: "block", Obj: 20, Attacker: 12},
			{Index: 3, Kind: "block", Obj: 21, Attacker: 10},
		}}
	for seed := uint64(0); seed < 50; seed++ {
		in, err := NewBot(seed).Decide(ctx, view.View{}, blockers)
		if err != nil {
			t.Fatalf("seed %d: blockers returned an error: %v", seed, err)
		}
		if err := blockers.Validate(in); err != nil {
			t.Fatalf("seed %d: blockers intent failed Validate: %v", seed, err)
		}
		used := map[state.ObjID]bool{}
		for _, c := range in.Choices {
			obj := blockers.Options[c].Obj
			if used[obj] {
				t.Fatalf("seed %d: blockers = %+v assigned blocker %d to more than one attacker", seed, in, obj)
			}
			used[obj] = true
		}
	}
}
