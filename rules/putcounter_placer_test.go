package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestCounterAddedValidSourceDistinguishesPlacer(t *testing.T) {
	e := newSeats(t, 2)
	source := e.G.AddObject(card(t, "Name:Counter watcher\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	target := e.G.AddObject(card(t, "Name:Counter target\nTypes:Creature\nPT:1/1\nOracle:x\n"), 1)
	for _, o := range []*state.Object{source, target} {
		o.Zone = state.ZBattlefield
		e.G.SetZone(state.ZBattlefield, o.Controller, append(e.G.Zone(state.ZBattlefield, o.Controller), o.ID))
	}
	if source.Zone != state.ZBattlefield || target.Zone != state.ZBattlefield || source.Controller == target.Controller {
		t.Fatal("precondition: watcher and counter recipient must be distinct battlefield players")
	}
	trig := cards.Trigger{Mode: "CounterPlayerAddedAll", Params: map[string]string{"ValidSource": "You"}}
	for _, tc := range []struct {
		placer state.PlayerID
		want   bool
	}{{0, true}, {1, false}} {
		previous := e.SetCounterAdder(tc.placer)
		ev := events.Event{Kind: events.CounterChange, Obj: target.ID, Counter: "P1P1", Amount: 1}
		e.emit(ev)
		matched := e.counterPlayerAddedAllMatches(trig, source.ID, ev, nil)
		e.SetCounterAdder(previous)
		if matched != tc.want {
			t.Errorf("ValidSource You with placer %d matched=%v, want %v", tc.placer, matched, tc.want)
		}
	}
}
