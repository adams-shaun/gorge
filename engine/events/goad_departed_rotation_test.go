package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestDepartedGoadFollowsTheTurnRotation drives the turn events of one
// four-seat advance from p0's turn 5 and reads whether the departed goader's
// goad survived it (CR 800.4m: it ends exactly when the ordinary rotation
// passes that seat's slot).
func TestDepartedGoadFollowsTheTurnRotation(t *testing.T) {
	skip := func(p state.PlayerID) Event { return Event{Kind: SkipTurn, Player: p, Amount: -1} }
	grant := func(p state.PlayerID) Event { return Event{Kind: ExtraTurn, Player: p, Amount: 1} }
	take := func(p state.PlayerID) Event { return Event{Kind: ExtraTurn, Player: p, Amount: -1} }
	drop := func(p state.PlayerID) Event {
		return Event{Kind: ExtraTurn, Player: p, Amount: -1, Text: ExtraTurnSkippedText}
	}
	turn := func(p state.PlayerID) Event { return Event{Kind: TurnChange, Player: p} }
	cases := []struct {
		name   string
		goader state.PlayerID // departed
		events []Event
		kept   bool
	}{
		{"ordinary advance over the slot", 1, []Event{turn(2)}, false},
		{"ordinary advance short of the slot", 3, []Event{turn(1)}, true},
		{"every other seat skipped, back to the same seat", 1, []Event{skip(2), skip(3), turn(0)}, false},
		{"skips wrap past the base into a second cycle", 3, []Event{skip(1), skip(2), skip(0), turn(1)}, false},
		{"own extra turn", 1, []Event{grant(0), take(0), turn(0)}, true},
		{"another seat's extra turn", 1, []Event{grant(2), take(2), turn(2)}, true},
		{"rotation resumes after the base, short of the slot", 3,
			[]Event{grant(0), take(0), turn(0), turn(1)}, true},
		{"rotation resumes after the base, not the extra holder", 1,
			[]Event{grant(2), take(2), turn(2), turn(2)}, false},
		{"leaving an extra turn short of the slot", 3,
			[]Event{grant(0), take(0), turn(0), turn(2)}, true},
		{"consecutive extra turns keep the base", 1,
			[]Event{grant(3), grant(2), take(2), turn(2), take(3), turn(3), turn(2)}, false},
		{"departed seat's grant consumed without a turn", 1, []Event{grant(1), take(1), turn(2)}, false},
		{"skipped extra turn begins nothing", 1, []Event{grant(2), drop(2), turn(2)}, false},
		{"skipped extra turn does not move the rotation", 3,
			[]Event{grant(2), skip(2), drop(2), turn(1)}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := state.NewGame([]string{"a", "b", "c", "d"})
			id := g.AddObject(bearCard(), 0).ID
			g.Obj(id).Zone = state.ZBattlefield
			g.SetZone(state.ZBattlefield, 0, []state.ObjID{id})
			l := NewLog(1)
			g.Turn = 5
			g.Players[c.goader].Lost = true
			Emit(g, l, Event{Kind: Goad, Obj: id, Player: c.goader})
			if len(g.Obj(id).Goads) != 1 {
				t.Fatalf("precondition: goads = %v, want the one goad", g.Obj(id).Goads)
			}
			// Every event is applied to a fresh clone, so the rotation
			// bookkeeping has to survive Clone.
			for _, ev := range c.events {
				g = g.Clone()
				Emit(g, l, ev)
			}
			if kept := len(g.Obj(id).Goads) == 1; kept != c.kept {
				t.Errorf("goad kept = %v, want %v", kept, c.kept)
			}
		})
	}
}
