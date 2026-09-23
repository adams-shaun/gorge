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

// TestPoisonIgnoresObjectTargets pins effPoison's non-player guard. The
// Defined$ list deliberately MIXES an object target with a player target so
// the leaf is failure-proof in BOTH directions: dropping the !t.IsPlayer
// guard emits TWO events (PlayerOf falls back to the object's CONTROLLER,
// an in-range seat the bounds check cannot catch), and a no-op effPoison
// emits NONE. An object-only assertion would pass against a
// no-op implementation, so it is not written that way.
func TestPoisonIgnoresObjectTargets(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	effPoison(h, &Ctx{Controller: 0, Source: ids["myBear"],
		Targets: []state.Target{
			{Obj: ids["theirBig"]},
			{Player: 1, IsPlayer: true},
		}},
		sa(t, "DB$ Poison | ValidTgts$ Player | Num$ 2"))
	if len(h.log) != 1 {
		t.Fatalf("log = %+v, want exactly one event (the object target is skipped, the player target is not)", h.log)
	}
	ev := h.log[0]
	if ev.Kind != events.PlayerCounterChange || ev.Player != 1 || ev.Counter != "POISON" || ev.Amount != 2 {
		t.Fatalf("event = %+v, want a +2 POISON PlayerCounterChange on player 1", ev)
	}
}
