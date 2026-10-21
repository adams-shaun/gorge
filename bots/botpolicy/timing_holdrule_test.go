package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

func holdBoard(timing TimingRules, c Card) Board {
	b := Board{MyTurn: true, IsMain: true, Step: state.StepMain1, Timing: timing,
		Cards: TableOf(map[state.ObjID]Card{10: c}),
	}
	b.Life.Set(0, 20)
	b.Life.Set(1, 20)
	return b
}

func holdDecision() *decision.Decision {
	return &decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority}
}

func holdNow(b Board) bool {
	return b.timingNow(holdDecision(), decision.Option{Kind: "cast", Obj: 10})
}

func TestHoldRuleWindows(t *testing.T) {
	instant := Card{InstantSpeed: true, Castable: true, CMC: 1, ManaCost: "U"}
	trick := instant
	trick.CombatTrick = true
	counter := instant
	counter.Counter = true
	burn := instant
	burn.RemovalKind, burn.RemovalDamage, burn.RemovalTargets = "damage", 3, "player"
	for _, tc := range []struct {
		name  string
		card  Card
		build func(Board) Board
		want  bool
	}{
		{name: "own main 1", card: instant},
		{name: "own main 2", card: instant, build: func(b Board) Board { b.Step = state.StepMain2; return b }},
		{name: "opponent main 1", card: instant, build: func(b Board) Board { b.MyTurn = false; return b }},
		{name: "opponent main 2", card: instant, build: func(b Board) Board { b.MyTurn = false; b.Step = state.StepMain2; return b }, want: true},
		{name: "opponent end", card: instant, build: func(b Board) Board { b.MyTurn = false; b.Step = state.StepEnd; return b }, want: true},
		{name: "own declare attackers trick", card: trick, build: func(b Board) Board { b.Step = state.StepDeclareAttackers; return b }},
		{name: "own declare blockers trick", card: trick, build: func(b Board) Board { b.Step = state.StepDeclareBlockers; return b }, want: true},
		{name: "own combat damage trick", card: trick, build: func(b Board) Board { b.Step = state.StepCombatDamage; return b }, want: true},
		{name: "own end combat trick", card: trick, build: func(b Board) Board { b.Step = state.StepEndCombat; return b }, want: true},
		{name: "opponent declare blockers trick", card: trick, build: func(b Board) Board { b.MyTurn = false; b.Step = state.StepDeclareBlockers; return b }, want: true},
		{name: "non-trick in combat", card: instant, build: func(b Board) Board { b.Step = state.StepDeclareBlockers; return b }},
		{name: "foreign top", card: instant, build: func(b Board) Board {
			b.Stack = []StackEntry{{ID: 50, Controller: 1, IsSpell: true, CMC: 3}}
			return b
		}, want: true},
		{name: "foreign ability top", card: instant, build: func(b Board) Board {
			b.Stack = []StackEntry{{ID: 50, Controller: 1}}
			return b
		}, want: true},
		{name: "own top", card: instant, build: func(b Board) Board {
			b.Stack = []StackEntry{{ID: 50, Controller: 0, IsSpell: true, CMC: 3}}
			return b
		}},
		{name: "counter foreign top", card: counter, build: func(b Board) Board {
			b.Stack = []StackEntry{{ID: 50, Controller: 1, IsSpell: true, CMC: 3}}
			return b
		}, want: true},
		{name: "counter own top at opponent end", card: counter, build: func(b Board) Board {
			b.MyTurn = false
			b.Step = state.StepEnd
			b.Stack = []StackEntry{{ID: 50, Controller: 0, IsSpell: true, CMC: 3}}
			return b
		}},
		{name: "lethal burn", card: burn, build: func(b Board) Board { b.Life.Set(1, 3); return b }, want: true},
		{name: "nonlethal burn", card: burn, build: func(b Board) Board { b.Life.Set(1, 4); return b }},
		{name: "sorcery", card: Card{Castable: true, CMC: 1}, want: true},
		{name: "land", card: Card{InstantSpeed: true, Land: true, Castable: true}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := holdBoard(TimingRules{HoldInstant: true}, tc.card)
			if tc.build != nil {
				b = tc.build(b)
			}
			if got := holdNow(b); got != tc.want {
				t.Fatalf("timingNow = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHoldRuleDefaultUntouched(t *testing.T) {
	instant := Card{InstantSpeed: true, Castable: true, CMC: 1, ManaCost: "U"}
	if !holdNow(holdBoard(TimingRules{}, instant)) {
		t.Fatal("zero TimingRules held an instant")
	}
	if !holdBoard(TimingRules{HoldInstant: true}, instant).timedCard(instant) {
		t.Fatal("HoldInstant did not make the instant a timed tap candidate")
	}
	if holdBoard(TimingRules{}, instant).timedCard(instant) {
		t.Fatal("zero TimingRules made the instant a timed tap candidate")
	}
}

func TestHoldRuleTimedTap(t *testing.T) {
	build := func(timing TimingRules) Board {
		b := Board{MyTurn: false, Step: state.StepEnd, Timing: timing,
			Cards: TableOf(map[state.ObjID]Card{
				10: {InstantSpeed: true, CMC: 2, ManaCost: "U U", Castable: true},
				30: nowIsland(),
				31: nowIsland(),
			}),
		}
		return b
	}
	opts := []decision.Option{
		{Index: 0, Kind: "activate", Obj: 30},
		{Index: 1, Kind: "activate", Obj: 31},
		{Index: 2, Kind: "pass"},
	}
	off, d := castDecision(build(TimingRules{}), opts)
	if d.Options[off].Kind != "pass" {
		t.Fatalf("rule off: chose %q, want the pass (2)", d.Options[off].Kind)
	}
	on, d := castDecision(build(TimingRules{HoldInstant: true}), opts)
	if d.Options[on].Kind != "activate" {
		t.Fatalf("rule on: chose %q, want the first Island activation (0)", d.Options[on].Kind)
	}
}
