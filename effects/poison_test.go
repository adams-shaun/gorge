package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestPoisonEmitsSignedPlayerCounterChange(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	g.Players[1].AddCounter("POISON", 3)
	effPoison(h, &Ctx{Controller: 0, Source: ids["myBear"],
		Targets: []state.Target{{Player: 1, IsPlayer: true}}},
		sa(t, "DB$ Poison | ValidTgts$ Player | Num$ -3"))
	if len(h.log) != 1 || h.log[0].Kind != events.PlayerCounterChange {
		t.Fatalf("log = %+v, want one PlayerCounterChange", h.log)
	}
	if got := h.log[0].Amount; got != -3 {
		t.Fatalf("poison amount = %d, want -3", got)
	}
	if h.log[0].Player != 1 || h.log[0].Counter != "POISON" {
		t.Fatalf("poison event = %+v, want player 1 POISON", h.log[0])
	}
}

func TestPoisonIgnoresObjectTargets(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	effPoison(h, &Ctx{Controller: 0, Source: ids["myBear"],
		Targets: []state.Target{{Obj: ids["theirBig"]}}},
		sa(t, "DB$ Poison | ValidTgts$ Player | Num$ 2"))
	if len(h.log) != 0 {
		t.Fatalf("log = %+v, want no event for an invalid player target", h.log)
	}
}
