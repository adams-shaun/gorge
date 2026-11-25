package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEffectLosesGameTriggerFires pins the delayed walk's LosesGame matcher: a
// Duration$ Permanent Effect that remembers a player (Miss Highwater's
// DBEffect RememberObjects$ TriggeredTarget, Triggers$ LoseGameTrigger) arms a
// LosesGame registration whose ValidPlayer$ Player.IsRemembered reads the
// Effect's own capture. When the remembered player loses in a three-seat game
// -- a loss that does not end the game (CR 104.3, 800.4a) -- the trigger must
// reach the stack; an unrelated event must neither fire it nor consume the
// registration.
func TestEffectLosesGameTriggerFires(t *testing.T) {
	t.Parallel()
	e := newSeats(t, 3)
	loser := state.PlayerID(1)
	if len(e.G.Players) != 3 || e.G.Players[loser].Lost {
		t.Fatalf("precondition: want three living seats, got %d players (seat %d lost: %v)",
			len(e.G.Players), loser, e.G.Players[loser].Lost)
	}
	src := onBoard(t, e, 0, "Name:Highwater promise\nTypes:Enchantment\n"+
		"SVar:LoseGameTrigger:Mode$ LosesGame | ValidPlayer$ Player.IsRemembered | TriggerZones$ Command | Execute$ TrigPain\n"+
		"SVar:TrigPain:DB$ LoseLife | Defined$ You | LifeAmount$ 2\nOracle:x\n")
	if o := e.G.Obj(src); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Effect source %d must be on the battlefield: %+v", src, o)
	}
	// The Effect remembers the player who took the contract (the damaged
	// player, Forge's TriggeredTarget), exactly Miss Highwater's DBEffect.
	effects.Resolve(e, effects.NewCtxPtr(src, 0, effects.CtxInit{
		TriggerContext: effects.TriggerContext{TriggerTarget: state.Target{IsPlayer: true, Player: loser}},
	}), &cards.SA{Kind: "DB", API: "Effect", Params: map[string]string{
		"Triggers": "LoseGameTrigger", "Duration": "Permanent", "RememberObjects": "TriggeredTarget",
	}})
	if len(e.G.Delayed) != 1 {
		t.Fatalf("precondition: want one armed delayed trigger, got %d: %+v", len(e.G.Delayed), e.G.Delayed)
	}
	dt := e.G.Delayed[0]
	if dt.EventMode != "LosesGame" || !dt.EffectRepeat {
		t.Fatalf("precondition: registration = mode %q repeat %v, want LosesGame and a recurring Effect trigger",
			dt.EventMode, dt.EffectRepeat)
	}
	remembered := false
	for _, tg := range dt.Remembered {
		if tg.IsPlayer && tg.Player == loser {
			remembered = true
		}
	}
	if !remembered {
		t.Fatalf("precondition: registration does not remember seat %d: %+v", loser, dt.Remembered)
	}

	delayedPushes := func() int {
		return countEvents(e, func(ev events.Event) bool { return ev.Kind == events.DelayedPush })
	}
	if n := delayedPushes(); n != 0 {
		t.Fatalf("precondition: %d delayed triggers already on the stack", n)
	}
	// An unrelated event is not a loss: it must neither fire nor consume the
	// registration.
	e.emit(events.Event{Kind: events.Note, Text: "unrelated event"})
	e.putTriggersOnStack()
	if n := delayedPushes(); n != 0 {
		t.Fatalf("an unrelated event fired the LosesGame trigger: %d DelayedPush events", n)
	}
	if len(e.G.Delayed) != 1 {
		t.Fatalf("an unrelated event consumed the registration: %+v", e.G.Delayed)
	}

	// The remembered player's loss is the triggering event (CR 603.2).
	if !e.playerLoses(loser, loseReasonEffect, "remembered loss") {
		t.Fatal("precondition: the remembered player's loss did not happen")
	}
	e.putTriggersOnStack()
	if n := delayedPushes(); n != 1 {
		t.Fatalf("the remembered player's loss queued %d DelayedPush events, want 1", n)
	}
	before := e.G.Players[0].Life
	drainStack(t, e, 8)
	if got := e.G.Players[0].Life; got != before-2 {
		t.Fatalf("the trigger reached the stack but its Execute$ body never resolved: controller life %d, want %d", got, before-2)
	}
}
