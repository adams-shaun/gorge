// This file is package view (internal), not view_test: describeFixture
// reuses identity_test.go's boltSrc, an unexported package-view constant.
// TestDescribeIsIdenticalAcrossTwoRunsOfTheSameMatch lives in
// describe_replay_test.go instead, under package view_test, for the same
// reason visibility_test.go does (see that file's own doc comment): it
// needs seat.Bot to drive a real game, and seat imports view (bot.go takes
// a view.View) — an internal test file that also imported seat would be a
// genuine import cycle for Go's test tooling (view[.test] -> seat ->
// view), not a style choice. That test touches no package-private symbol,
// so package view_test costs it nothing.
package view

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func describeFixture(t *testing.T) (*state.Game, state.ObjID, state.ObjID) {
	t.Helper()
	bear, _ := cards.ParseBytes("b.txt", []byte("Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"))
	bolt, _ := cards.ParseBytes("l.txt", []byte(boltSrc))
	g := state.NewGame([]string{"Ann", "Bob"})
	b := g.AddObject(bear, 0)
	l := g.AddObject(bolt, 1)
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{b.ID})
	b.Zone = state.ZBattlefield
	g.SetZone(state.ZHand, 1, []state.ObjID{l.ID})
	l.Zone = state.ZHand
	g.Players[0].Life = 17
	return g, b.ID, l.ID
}

func TestDescribeTemplates(t *testing.T) {
	g, bear, bolt := describeFixture(t)
	cases := []struct {
		name string
		ev   events.Event
		want string
	}{
		{"game start", events.Event{Kind: events.GameStart, Amount: 4}, "Game starts with 4 players"},
		{"shuffle", events.Event{Kind: events.Shuffle, Player: 1}, "Bob shuffles their library"},
		{"move", events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard}, "Bear #1 moves from battlefield to graveyard"},
		{"hidden draw", events.Event{Kind: events.Draw, Player: 0}, "Ann draws a card"},
		{"visible draw", events.Event{Kind: events.Draw, Player: 1, Obj: bolt}, "Bob draws Bolt #2"},
		{"life gain", events.Event{Kind: events.LifeChange, Player: 0, Amount: 3}, "Ann gains 3 life (17)"},
		{"life loss", events.Event{Kind: events.LifeChange, Player: 0, Amount: -2}, "Ann loses 2 life (17)"},
		{"damage to creature", events.Event{Kind: events.Damage, Obj: bear, Amount: 2}, "Bear #1 takes 2 damage"},
		{"damage to player", events.Event{Kind: events.Damage, Player: 1, Amount: 3}, "Bob takes 3 damage"},
		{"tap", events.Event{Kind: events.Tap, Obj: bear}, "Bear #1 taps"},
		{"untap", events.Event{Kind: events.Untap, Obj: bear}, "Bear #1 untaps"},
		{"step", events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers}, "Step: declare-attackers"},
		{"turn", events.Event{Kind: events.TurnChange, Player: 1, Amount: 7}, "Turn 7: Bob"},
		{"priority", events.Event{Kind: events.Priority, Player: 0}, "Ann has priority"},
		{"cast", events.Event{Kind: events.PutOnStack, Player: 1, Obj: bolt}, "Bob casts Bolt #2"},
		{"resolve", events.Event{Kind: events.Resolve, Obj: bolt}, "Bolt #2 resolves"},
		{"mana add", events.Event{Kind: events.ManaAdd, Player: 0, Counter: "G", Amount: 2}, "Ann adds {G}{G}"},
		{"mana spend", events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: -1}, "Ann spends {R}"},
		{"mana clear", events.Event{Kind: events.ManaClear, Player: 0}, "Ann's mana pool empties"},
		{"counter add", events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: 2}, "Bear #1 gets 2 P1P1 counters"},
		{"counter remove", events.Event{Kind: events.CounterChange, Obj: bear, Counter: "M1M1", Amount: -1}, "Bear #1 loses 1 M1M1 counter"},
		{"attackers", events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear}}, "Bear #1 attacks Bob"},
		{"no attackers", events.Event{Kind: events.DeclareAttackers, Player: 1}, "No attackers"},
		{"blockers", events.Event{Kind: events.DeclareBlockers, Pairs: [][2]state.ObjID{{bear, bolt}}}, "Bolt #2 blocks Bear #1"},
		{"no blockers", events.Event{Kind: events.DeclareBlockers}, "No blocks"},
		{"player lost", events.Event{Kind: events.PlayerLost, Player: 0}, "Ann loses the game"},
		{"win", events.Event{Kind: events.GameOver, Player: 1}, "Bob wins the game"},
		{"draw game", events.Event{Kind: events.GameOver, Amount: 1}, "The game is a draw"},
		{"ask", events.Event{Kind: events.DecisionAsk, Player: 0, Text: "priority"}, "Ann is asked: priority"},
		{"answer", events.Event{Kind: events.DecisionMade, Player: 0, Text: "priority:[2]"}, "Ann answers priority:[2]"},
		{"note", events.Event{Kind: events.Note, Text: "Bob reveals Bolt"}, "Bob reveals Bolt"},
		{"redacted note", events.Event{Kind: events.Note, Secret: true, Player: 1}, "Bob looks at hidden cards"},
		{"land", events.Event{Kind: events.LandPlayed, Player: 0}, "Ann plays a land"},
		{"target player", events.Event{Kind: events.TargetsChosen, Obj: bolt, Player: 0, Amount: 1}, "Bolt #2 targets Ann"},
		{"target objects", events.Event{Kind: events.TargetsChosen, Obj: bolt, IDs: []state.ObjID{bear}}, "Bolt #2 targets Bear #1"},
		{"flip", events.Event{Kind: events.FlipFace, Obj: bear, Amount: 1}, "Bear #1 turns to face 1"},
		{"clock", events.Event{Kind: events.ClockTick}, ""},
		{"trigger", events.Event{Kind: events.TriggerPush, Player: 0, Obj: bear}, "Bear #1 triggers"},
		{"end combat", events.Event{Kind: events.EndCombatReset}, "Combat ends"},
		{"unknown kind", events.Event{Kind: 250}, "unknown event"},
		{"unknown seat", events.Event{Kind: events.Priority, Player: 9}, "seat 9 has priority"},
		{"unknown object", events.Event{Kind: events.Tap, Obj: 77}, "#77 taps"},
	}
	for _, tc := range cases {
		if got := Describe(g, tc.ev); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestDescribeCoversEveryKind(t *testing.T) {
	g, bear, _ := describeFixture(t)
	for k := events.GameStart; k <= events.EndCombatReset; k++ {
		ev := events.Event{Kind: k, Player: 0, Obj: bear, Amount: 1, IDs: []state.ObjID{bear}, Pairs: [][2]state.ObjID{{bear, bear}}}
		got := Describe(g, ev)
		if k == events.ClockTick {
			if got != "" {
				t.Errorf("ClockTick should describe as empty, got %q", got)
			}
			continue
		}
		if got == "" || got == "unknown event" {
			t.Errorf("kind %s (%d) has no description", k, k)
		}
	}
}

func TestDescribeNeverPanics(t *testing.T) {
	g, _, _ := describeFixture(t)
	for _, gg := range []*state.Game{nil, g, state.NewGame(nil)} {
		for k := events.Kind(0); k < 40; k++ {
			ev := events.Event{Kind: k, Player: 250, Obj: 1 << 30, Amount: -7, Step: 99, From: 99, To: 99,
				IDs: []state.ObjID{0, 1 << 30}, Pairs: [][2]state.ObjID{{0, 1 << 30}}}
			_ = Describe(gg, ev)
		}
	}
}
