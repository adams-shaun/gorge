package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestPlayerCountPlayersCounters(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	h.Emit(events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "RAD", Amount: 2})
	h.Emit(events.Event{Kind: events.PlayerCounterChange, Player: 1, Counter: "RAD", Amount: 3})
	for _, tc := range []struct {
		expr string
		want int32
	}{
		{"PlayerCountPlayers$Counters.RAD", 5},
		{"PlayerCountPlayers$Counters.ALL", 5},
	} {
		if got, ok := EvalCountOK(h, c, tc.expr); !ok || got != tc.want {
			t.Errorf("EvalCountOK(%q)=(%d,%v), want (%d,true)", tc.expr, got, ok, tc.want)
		}
	}
}
