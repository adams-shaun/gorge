package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// counterBoard builds the A6b board: a castable instant counter (card 10)
// in hand, the given stack, and a pool of {U}{U}. The counter costs {U}{U}
// and is always castable, so every OFF/ON difference is the timing rule.
func counterBoard(timing TimingRules, top StackEntry) Board {
	b := Board{MyTurn: false, Step: state.StepUpkeep, Timing: timing,
		Cards: TableOf(map[state.ObjID]Card{
			10: {Counter: true, InstantSpeed: true, CMC: 2, ManaCost: "U U", Castable: true},
		}),
		Stack: []StackEntry{top},
	}
	b.Pool[state.MU] = 2
	return b
}

func counterOptions() []decision.Option {
	return []decision.Option{
		{Index: 0, Kind: "cast", Obj: 10},
		{Index: 1, Kind: "pass"},
	}
}

// TestCounterTopSkipsOneDrop is A6b's firing evidence: the armed rule holds
// a counter when the foreign spell on top has mana value 1, while CMC 3 and
// CMC 0 are countered. The rule off casts in every case.
func TestCounterTopSkipsOneDrop(t *testing.T) {
	for _, tc := range []struct {
		cmc  int32
		want int
	}{
		{cmc: 1, want: 1},
		{cmc: 3, want: 0},
		{cmc: 0, want: 0},
	} {
		top := StackEntry{ID: 50, Controller: 1, IsSpell: true, CMC: tc.cmc}
		off, _ := castDecision(counterBoard(TimingRules{}, top), counterOptions())
		if off != 0 {
			t.Fatalf("cmc %d, rule off: chose option %d, want the counter cast (0)", tc.cmc, off)
		}
		on, d := castDecision(counterBoard(TimingRules{CounterTop: true}, top), counterOptions())
		if on != tc.want {
			t.Fatalf("cmc %d, rule on: chose %q (option %d), want option %d",
				tc.cmc, d.Options[on].Kind, on, tc.want)
		}
	}
}

// TestCounterTopNeedsForeignTop: the rule reads the TOP of the stack, so an
// opponent's spell below the seat's own spell is not a counter target.
func TestCounterTopNeedsForeignTop(t *testing.T) {
	b := counterBoard(TimingRules{CounterTop: true}, StackEntry{ID: 50, Controller: 1, IsSpell: true, CMC: 3})
	b.Stack = append(b.Stack, StackEntry{ID: 51, Controller: 0, IsSpell: true, CMC: 2})
	if got, _ := castDecision(b, counterOptions()); got != 1 {
		t.Fatalf("rule on: chose option %d, want the pass (1) when the seat's own spell is on top", got)
	}
	b.Timing = TimingRules{}
	if got, _ := castDecision(b, counterOptions()); got != 0 {
		t.Fatalf("rule off: chose option %d, want the counter cast (0) at a foreign stack", got)
	}
}

// TestCounterTopTargetsTop: the target half picks the spell just below the
// counter being cast, not the highest-CMC foreign spell the ordinary removal
// ranking leads with.
func TestCounterTopTargetsTop(t *testing.T) {
	build := func(timing TimingRules) Board {
		return Board{Timing: timing,
			Stack: []StackEntry{
				{ID: 50, Controller: 1, IsSpell: true, CMC: 5},
				{ID: 52, Controller: 1, IsSpell: true, CMC: 2},
				{ID: 60, Controller: 0, IsSpell: true},
			},
		}
	}
	opts := []decision.Option{
		{Index: 0, Kind: "permanent", Obj: 50},
		{Index: 1, Kind: "permanent", Obj: 52},
	}
	target := func(b Board) int {
		d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KTarget, Min: 1, Max: 1, Source: 60,
			TargetEffect: &decision.TargetEffect{API: "Counter"}, Options: opts}
		in := Decide(b, &d, rng(1))
		if len(in.Choices) != 1 {
			return -1
		}
		return in.Choices[0]
	}
	if got := target(build(TimingRules{})); got != 0 {
		t.Fatalf("rule off: chose option %d, want the highest-CMC foreign spell (0)", got)
	}
	if got := target(build(TimingRules{CounterTop: true})); got != 1 {
		t.Fatalf("rule on: chose option %d, want the top spell below the counter (1)", got)
	}
}

// TestCounterTopTimedTap: outside a main phase the armed rule floats mana
// toward the counter, but only when its own cast gate allows the cast (a
// CMC-1 top is still held).
func TestCounterTopTimedTap(t *testing.T) {
	build := func(timing TimingRules, topCMC int32) Board {
		b := Board{MyTurn: false, Step: state.StepUpkeep, Timing: timing,
			Cards: TableOf(map[state.ObjID]Card{
				10: {Counter: true, InstantSpeed: true, CMC: 2, ManaCost: "U U", Castable: true},
				30: nowIsland(),
				31: nowIsland(),
			}),
			Stack: []StackEntry{{ID: 50, Controller: 1, IsSpell: true, CMC: topCMC}},
		}
		return b
	}
	opts := []decision.Option{
		{Index: 0, Kind: "activate", Obj: 30},
		{Index: 1, Kind: "activate", Obj: 31},
		{Index: 2, Kind: "pass"},
	}
	off, d := castDecision(build(TimingRules{}, 3), opts)
	if d.Options[off].Kind != "pass" {
		t.Fatalf("rule off: chose %q, want the pass (2) with no main-phase tap gate", d.Options[off].Kind)
	}
	on, d := castDecision(build(TimingRules{CounterTop: true}, 3), opts)
	if d.Options[on].Kind != "activate" {
		t.Fatalf("rule on: chose %q, want the first Island activation (0)", d.Options[on].Kind)
	}
	held, d := castDecision(build(TimingRules{CounterTop: true}, 1), opts)
	if d.Options[held].Kind != "pass" {
		t.Fatalf("rule on, CMC-1 top: chose %q, want the pass (2): no mana may float toward a held counter", d.Options[held].Kind)
	}
}

// TestCounterTopDefaultUntouched pins the opt-in boundary: the zero
// TimingRules is exactly the rule-off board.
func TestCounterTopDefaultUntouched(t *testing.T) {
	top := StackEntry{ID: 50, Controller: 1, IsSpell: true, CMC: 1}
	off, _ := castDecision(counterBoard(TimingRules{}, top), counterOptions())
	zero, _ := castDecision(counterBoard(TimingRules{}, top), counterOptions())
	if off != zero || off != 0 {
		t.Fatalf("zero TimingRules chose %d, rule off chose %d, want both the cast (0)", zero, off)
	}
}

// TestCounterTopHeldGetsNoMainTap is the main-phase half of castHeld: a
// counter with no foreign spell on the stack is not a tap need, so the
// priority window passes instead of floating mana toward a cast C8 refuses.
func TestCounterTopHeldGetsNoMainTap(t *testing.T) {
	build := func(timing TimingRules) Board {
		b := Board{IsMain: true, MyTurn: true, Step: state.StepMain1, Timing: timing,
			Cards: TableOf(map[state.ObjID]Card{
				10: {Counter: true, InstantSpeed: true, CMC: 2, ManaCost: "U U", Castable: true},
				30: nowIsland(),
			}),
		}
		return b
	}
	opts := []decision.Option{
		{Index: 0, Kind: "activate", Obj: 30},
		{Index: 1, Kind: "pass"},
	}
	off, d := castDecision(build(TimingRules{}), opts)
	if d.Options[off].Kind != "activate" {
		t.Fatalf("rule off: chose %q, want the Island activation (0)", d.Options[off].Kind)
	}
	on, d := castDecision(build(TimingRules{CounterTop: true}), opts)
	if d.Options[on].Kind != "pass" {
		t.Fatalf("rule on: chose %q, want the pass (1): the held counter must not float mana", d.Options[on].Kind)
	}
}
