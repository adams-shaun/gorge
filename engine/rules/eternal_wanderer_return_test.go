package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEternalWandererReturnsExiledCardAtOwnersNextEndStep drives the real
// corpus The Eternal Wanderer end to end: its +1 exiles seat 1's Grizzly
// Bears, the delayed return is gated on `ValidPlayer$ Player.IsTriggerRemembered`
// and must skip seat 0's end step (the card's owner is seat 1), then return the
// bear to seat 1's battlefield at the beginning of seat 1's next end step.
func TestEternalWandererReturnsExiledCardAtOwnersNextEndStep(t *testing.T) {
	t.Parallel()
	wanderer := corpusCard(t, "The Eternal Wanderer")
	bear := corpusCard(t, "Grizzly Bears")
	e, cfg := tokenReplGameSeats(t, 4711, []*cards.Card{wanderer}, []*cards.Card{bear})
	walker := moveSeededCard(t, e, 0, wanderer, state.ZBattlefield)
	victim := moveSeededCard(t, e, 1, bear, state.ZBattlefield)
	e.pending = nil

	// Preconditions: the walker is seat 0's planeswalker, the bear is seat
	// 1's own creature on seat 1's battlefield, and seat 0 is taking the
	// turn the +1 is activated in.
	wo, bo := e.G.Obj(walker), e.G.Obj(victim)
	if wo == nil || wo.Zone != state.ZBattlefield || wo.Controller != 0 {
		t.Fatalf("precondition: wanderer = %+v, want seat 0's battlefield permanent", wo)
	}
	if bo == nil || bo.Zone != state.ZBattlefield || bo.Controller != 1 || bo.Owner != 1 {
		t.Fatalf("precondition: bear = %+v, want seat 1's own battlefield creature", bo)
	}
	if e.G.Active != 0 {
		t.Fatalf("precondition: active seat = %d, want seat 0", e.G.Active)
	}

	addMana(t, e, 0, "")
	submitChoices(t, e, abilityOption(t, e, walker, 0).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("the +1 exile target ask = %+v, want a target decision", d)
	}
	idx := indexOfObjOption(d, victim)
	if idx < 0 {
		t.Fatalf("the +1 target ask does not offer seat 1's bear: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)

	if z := e.G.Obj(victim).Zone; z != state.ZExile {
		t.Fatalf("bear zone after the +1 = %s, want exile", z)
	}
	// Precondition for the player gate: the registration captured seat 1 (the
	// exiled card's owner) as a PLAYER entry, not just the card.
	reg := -1
	for i := range e.G.Delayed {
		if e.G.Delayed[i].ValidPlayer == "Player.IsTriggerRemembered" {
			reg = i
			break
		}
	}
	if reg < 0 {
		t.Fatalf("no Player.IsTriggerRemembered registration: %+v", e.G.Delayed)
	}
	capturedPlayer := false
	for _, target := range e.G.Delayed[reg].Remembered {
		if target.IsPlayer && target.Player == 1 {
			capturedPlayer = true
		}
	}
	if !capturedPlayer {
		t.Fatalf("registration capture %+v lacks seat 1 as a player", e.G.Delayed[reg].Remembered)
	}

	// Seat 0's end step must not be the bear's return: the bear's owner is
	// seat 1, and the registration waits for that player's next end step.
	seat0Turn := e.G.Turn
	ateotDriveToStep(t, e, seat0Turn, 0, state.StepEnd)
	for e.putTriggersOnStack() {
		answerTriggerOrders(t, e)
		passUntilStackEmpty(t, e, 20)
	}
	passUntilStackEmpty(t, e, 20)
	if z := e.G.Obj(victim).Zone; z != state.ZExile {
		t.Fatalf("bear zone at seat 0's end step = %s, want still exiled", z)
	}

	// Seat 1's next end step is the named event: the bear returns under its
	// owner's control and the one-shot registration is consumed.
	ateotDriveToStep(t, e, seat0Turn+1, 1, state.StepEnd)
	for e.putTriggersOnStack() {
		answerTriggerOrders(t, e)
		passUntilStackEmpty(t, e, 20)
	}
	passUntilStackEmpty(t, e, 20)
	bo = e.G.Obj(victim)
	if bo == nil || bo.Zone != state.ZBattlefield || bo.Controller != 1 {
		t.Fatalf("bear after seat 1's end step = %+v, want on seat 1's battlefield", bo)
	}
	for i := range e.G.Delayed {
		if e.G.Delayed[i].ValidPlayer == "Player.IsTriggerRemembered" {
			t.Fatalf("the return registration survived its one-shot fire: %+v", e.G.Delayed[i])
		}
	}
	replayCheck(t, e, cfg)
}
