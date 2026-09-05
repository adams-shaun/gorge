package rules

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// testBot is a line-for-line mirror of seat.Bot's policy (seat/bot.go's
// botDecide and clamp), duplicated here rather than imported: rules/fuzz_test.go
// is package rules, and importing seat -- which imports view -- runs the
// declared dependency order (cards -> state -> decision -> events ->
// effects -> rules -> view -> seat -> replay -> cmd/*) backwards into the
// package under test (Ruling F7). Task 26's acceptance harness drives the
// real seat.Bot instead; this copy exists solely so the rules package's own
// fuzz gate needs no import of anything above it. Keep the two in step: a
// policy change here without the matching change in seat/bot.go (or vice
// versa) is a bug.
type testBot struct {
	r *rand.Rand
}

func newTestBot(seed uint64) *testBot {
	return &testBot{r: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
}

// answer mirrors seat.Bot's botDecide exactly -- see that function's doc for
// the rationale of each case (Ruling P7): tap for mana only in a main phase
// (Ruling T25-b, fix round 1 -- seat.Bot reads this from the View's Phase;
// this package has no View, so its caller (rules/fuzz_test.go) computes
// isMain from e.G.Step.IsMain() on the line before calling answer), then
// land, then cast, then pass; attack with everything; target an opponent
// over yourself; block about half the time, never with the same blocker
// twice; order or accept/decline triggers with the bot's own rng; and, for
// anything else (or anything above that found nothing to pick), whatever
// clamp tops up with. Every access into d.Options is guarded against the
// list being empty, and clamp (Ruling T25-c, fix round 1) is the last thing
// every return does, so the intent this returns always validates against d
// for any Min/Max the wire format allows, not only today's shapes.
func (b *testBot) answer(isMain bool, d *decision.Decision) decision.Intent {
	in := decision.Intent{Seq: d.Seq, Player: d.Player}
	switch d.Kind {
	case decision.KPriority:
		if isMain {
			for _, o := range d.Options {
				if o.Kind == "activate" {
					in.Choices = []int{o.Index}
					return clamp(d, in)
				}
			}
		}
		for _, want := range [...]string{"play_land", "cast"} {
			for _, o := range d.Options {
				if o.Kind == want {
					in.Choices = []int{o.Index}
					return clamp(d, in)
				}
			}
		}
		// Ruling T25-g (fix round 2): explicitly pass here, before clamp
		// ever runs. This is the common case -- outside a main phase, or
		// with nothing affordable in one -- and legalActions
		// (rules/legal.go) lists every "activate" option before "pass", so
		// without this, clamp's blind first-unused-index top-up (needed
		// because a priority decision is always Min:1/Max:1, so falling
		// through with in.Choices empty is not itself a legal answer) would
		// reach for an activation and reintroduce I-1(b) through the
		// fallback path -- exactly what fix round 1 missed, because its own
		// synthetic regression decision carried a play_land option the loop
		// above matches unconditionally, a shape the live engine never
		// offers outside sorcery speed. Pass is offered on every priority
		// decision the engine emits; if it is somehow absent, this falls
		// through to the shared last resort below, same as any other kind.
		for _, o := range d.Options {
			if o.Kind == "pass" {
				in.Choices = []int{o.Index}
				return clamp(d, in)
			}
		}

	case decision.KTarget:
		for _, o := range d.Options {
			if o.Kind == "player" && o.Player != d.Player {
				in.Choices = []int{o.Index}
				return clamp(d, in)
			}
		}
		if len(d.Options) > 0 {
			in.Choices = []int{d.Options[0].Index}
			return clamp(d, in)
		}

	case decision.KAttackers:
		ch := make([]int, 0, len(d.Options))
		for _, o := range d.Options {
			ch = append(ch, o.Index)
		}
		in.Choices = ch
		return clamp(d, in)

	case decision.KBlockers:
		var ch []int
		used := map[state.ObjID]bool{} // membership only -- never ranged.
		for _, o := range d.Options {
			if !used[o.Obj] && b.r.IntN(2) == 0 {
				used[o.Obj] = true
				ch = append(ch, o.Index)
			}
		}
		in.Choices = ch
		return clamp(d, in)

	case decision.KTriggerOrder:
		if n := len(d.Options); n > 0 {
			// M3: build from o.Index like every other branch, not a bare
			// position literal -- identical today (Index == position
			// everywhere a real Decision is built, and invariant 5 checks
			// it), but this is the one branch that didn't say so.
			perm := make([]int, n)
			for i, o := range d.Options {
				perm[i] = o.Index
			}
			for i := n - 1; i > 0; i-- {
				j := b.r.IntN(i + 1)
				perm[i], perm[j] = perm[j], perm[i]
			}
			in.Choices = perm
			return clamp(d, in)
		}

	case decision.KTriggerOptional:
		if idx := b.r.IntN(2); idx < len(d.Options) {
			in.Choices = []int{d.Options[idx].Index}
			return clamp(d, in)
		}

	case decision.KChoose:
		if len(d.Options) == 0 {
			break
		}
		switch d.Options[0].Kind {
		case "x":
			in.Choices = []int{d.Options[len(d.Options)-1].Index} // the most it can pay for
		case "exile", "sacrifice":
			for i := 0; i < len(d.Options) && i < d.Max; i++ {
				in.Choices = append(in.Choices, d.Options[i].Index)
			}
		default: // yes/no (yes is first), name, type, number: the first offer
			in.Choices = []int{d.Options[0].Index}
		}
		return clamp(d, in)
	}

	// Last resort: pass if Min == 0 and one is offered; clamp below handles
	// everything else, including topping up to Min when nothing above (or
	// this) picked enough.
	if d.Min == 0 {
		for _, o := range d.Options {
			if o.Kind == "pass" {
				in.Choices = []int{o.Index}
				return clamp(d, in)
			}
		}
	}
	return clamp(d, in)
}

// clamp is seat/bot.go's clamp, mirrored line for line (Ruling F7): enforce
// [Min, Max] on top of whatever answer's switch (or its fallback) picked
// (Ruling T25-c). Truncate to at most Max, in the order already chosen,
// then -- if that leaves fewer than Min -- top up with the lowest-index
// unused options until Min is reached or none remain.
//
// Ruling T25-g (fix round 2): the top-up prefers an unused "pass" option
// over anything else. The KPriority branch above already returns "pass"
// explicitly before clamp ever runs, so this is defense in depth for
// whatever reaches here anyway (an absent "pass", or a future caller) --
// without it, a blind first-unused-index top-up would reach for whatever
// legalActions (rules/legal.go) happens to list first, which is every
// "activate" option, before "pass". That is precisely how fix round 1's own
// clamp reintroduced I-1(b): a Min:1 priority decision falling through with
// nothing chosen got topped up into an activation instead of a pass.
func clamp(d *decision.Decision, in decision.Intent) decision.Intent {
	max := d.Max
	if max < 0 {
		max = 0
	}
	if len(in.Choices) > max {
		in.Choices = append([]int(nil), in.Choices[:max]...)
	}
	min := d.Min
	if min < 0 {
		min = 0
	}
	if len(in.Choices) < min {
		have := make(map[int]bool, len(in.Choices)) // membership only -- never ranged.
		for _, c := range in.Choices {
			have[c] = true
		}
		for _, o := range d.Options {
			if len(in.Choices) >= min {
				break
			}
			if o.Kind == "pass" && !have[o.Index] {
				have[o.Index] = true
				in.Choices = append(in.Choices, o.Index)
			}
		}
		for _, o := range d.Options {
			if len(in.Choices) >= min {
				break
			}
			if !have[o.Index] {
				have[o.Index] = true
				in.Choices = append(in.Choices, o.Index)
			}
		}
	}
	return in
}

// TestTestBotChoosePolicy mirrors seat/bot_test.go's TestBotChoosePolicy
// against this package's own copy of the policy (Ruling F7).
func TestTestBotChoosePolicy(t *testing.T) {
	b := newTestBot(1)
	choose := func(kind string, n, min, max int) decision.Intent {
		d := decision.Decision{Player: 0, Kind: decision.KChoose, Min: min, Max: max}
		for i := 0; i < n; i++ {
			d.Options = append(d.Options, decision.Option{Index: i, Kind: kind, Label: kind})
		}
		if kind == "yes" {
			d.Options = []decision.Option{{Index: 0, Kind: "yes"}, {Index: 1, Kind: "no"}}
		}
		return b.answer(false, &d)
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

// TestTestBotPassesOutsideMainWithNoCastOrLandDrop mirrors
// seat/bot_test.go's TestBotPassesOutsideMainWithNoCastOrLandDrop against
// this package's own copy (Ruling F7): the engine's REAL shape for a
// non-main priority decision (rules/legal.go's legalActions never offers
// play_land or cast outside sorcery speed/an affordable instant) is one or
// more "activate" options followed by "pass" last -- no play_land, so the
// switch's un-gated play_land/cast loop cannot mask the fallback path the
// way fix round 1's own regression case accidentally did.
func TestTestBotPassesOutsideMainWithNoCastOrLandDrop(t *testing.T) {
	d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "activate", Obj: 100},
			{Index: 1, Kind: "activate", Obj: 101},
			{Index: 2, Kind: "pass"},
		}}
	b := newTestBot(1)
	in := b.answer(false, &d)
	if err := d.Validate(in); err != nil {
		t.Fatalf("isMain=false: intent %+v failed Validate: %v", in, err)
	}
	if len(in.Choices) != 1 || d.Options[in.Choices[0]].Kind != "pass" {
		t.Errorf("isMain=false: priority = %+v, want pass chosen -- not an activation -- outside a main phase", in)
	}
	// And inside a main phase, the same decision DOES prefer activate.
	in = b.answer(true, &d)
	if len(in.Choices) != 1 || d.Options[in.Choices[0]].Kind != "activate" {
		t.Errorf("isMain=true: priority = %+v, want an activation chosen", in)
	}
}

// TestTestBotTotalityUnderArbitraryMinMax is seat/bot_test.go's
// TestBotTotalityUnderArbitraryMinMax mirrored against this package's own
// copy of the policy (Ruling F7): 10,000 random decisions, random kind among
// all eight, 0-8 options, 0 <= Min <= Max <= len(Options) (so every one is
// satisfiable by construction) -- testBot.answer must validate against every
// one and never panic (I-4/Ruling T25-c).
func TestTestBotTotalityUnderArbitraryMinMax(t *testing.T) {
	kinds := []decision.Kind{decision.KPriority, decision.KTarget, decision.KAttackers,
		decision.KBlockers, decision.KMulligan, decision.KModes, decision.KTriggerOrder,
		decision.KTriggerOptional}
	optKinds := []string{"activate", "play_land", "cast", "pass", "player", "permanent",
		"attacker", "block", "trigger", "yes", "no", "keep", "mulligan", "whatever"}

	// A small, dependency-free xorshift, seeded once and consumed
	// sequentially: reproducible on its own, never math/rand's global
	// functions and never the bot's own rng.
	var s uint64 = 0x9E3779B97F4A7C15
	next := func(n int) int {
		s ^= s << 13
		s ^= s >> 7
		s ^= s << 17
		if n <= 0 {
			return 0
		}
		return int(s % uint64(n))
	}

	b := newTestBot(42)
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
		isMain := next(2) == 0

		var in decision.Intent
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("case %d: answer panicked on %+v (isMain=%v): %v", i, d, isMain, r)
				}
			}()
			in = b.answer(isMain, &d)
		}()
		if verr := d.Validate(in); verr != nil {
			t.Fatalf("case %d: intent %+v failed Validate against %+v (isMain=%v): %v", i, in, d, isMain, verr)
		}
	}
}
