package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The trigger-level gates Mode$ LifeGained honours (the r2 fix round):
// FirstTime$ (8 corpus lines over 7 files), PlayerTurn$ (5 lines over 5
// files) and the actionTriggerModes membership that makes ActivationLimit$
// (2 lines) apply at queue time. Before this round the mode matched every
// LifeChange unconditionally: a "once each turn" trigger fired every time
// and a "during your turn" trigger fired on opponents' turns.

// lifeGainedFixture seats a synthetic carrier with one T:Mode$ LifeGained
// trigger carrying the named extra params, beside the ValidPlayer$ You the
// corpus lines uniformly carry.
func lifeGainedFixture(t *testing.T, extraParams string) (*Engine, state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	id := onBoard(t, e, 0, "Name:Gainer\nTypes:Creature\n"+
		"T:Mode$ LifeGained | ValidPlayer$ You | "+extraParams+
		" | Execute$ TrigDraw\nSVar:TrigDraw:DB$ Draw\nOracle:x\n")
	return e, id
}

// FirstTime$ True ("whenever you gain life for the first time each turn")
// admits exactly the first gain of that player this turn. Synthetic pin: the
// once-per-turn contract is exactly one, so a second same-turn gain must not
// queue a second trigger.
func TestLifeGainedFirstTimeFiresOnceEachTurn(t *testing.T) {
	e, src := lifeGainedFixture(t, "FirstTime$ True")
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 2})
	if len(e.pendingTriggers) != 1 || e.pendingTriggers[0].Source != src {
		t.Fatalf("first gain of the turn queued %#v, want exactly one trigger from %d",
			e.pendingTriggers, src)
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 1})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("second same-turn gain queued %#v, want no second trigger (FirstTime$ True)",
			e.pendingTriggers)
	}
}

// FirstTime$ on a REAL corpus carrier: Attended Healer ("Whenever you gain
// life for the first time each turn, create a 1/1 white Cat"). Two same-turn
// gains queue one trigger, not two.
func TestAttendedHealerFirstTimeLifeGainedQueuesOnce(t *testing.T) {
	e := layerEngine(t)
	src := onBoardCard(t, e, 0, corpusCard(t, "Attended Healer"))
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 3})
	if len(e.pendingTriggers) != 1 || e.pendingTriggers[0].Source != src {
		t.Fatalf("first gain queued %#v, want exactly one Attended Healer trigger", e.pendingTriggers)
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 1})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("second same-turn gain queued %#v, want no second trigger", e.pendingTriggers)
	}
}

// PlayerTurn$ True ("whenever you gain life during your turn"): a gain on an
// opponent's turn queues nothing; the same gain on the controller's turn
// queues exactly one. Real corpus carrier: Vampire Scrivener's LifeGained
// line (its LifeLost twin shares the gate but does not match a gain).
func TestVampireScrivenerLifeGainedPlayerTurnGate(t *testing.T) {
	e := layerEngine(t)
	src := onBoardCard(t, e, 0, corpusCard(t, "Vampire Scrivener"))
	e.G.Active = 1
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 2})
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("gain on an opponent's turn queued %#v, want none (PlayerTurn$ True)",
			e.pendingTriggers)
	}
	e.G.Active = 0
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 2})
	if len(e.pendingTriggers) != 1 || e.pendingTriggers[0].Source != src {
		t.Fatalf("gain on the controller's turn queued %#v, want exactly one trigger from %d",
			e.pendingTriggers, src)
	}
}

// ActivationLimit$ applies at queue time now the mode is in
// actionTriggerModes: "this ability triggers only once each turn" queues the
// first activation and blocks the second.
func TestLifeGainedActivationLimitCapsQueueTime(t *testing.T) {
	e, src := lifeGainedFixture(t, "ActivationLimit$ 1")
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 2})
	if len(e.pendingTriggers) != 1 || e.pendingTriggers[0].Source != src {
		t.Fatalf("first activation queued %#v, want exactly one trigger from %d",
			e.pendingTriggers, src)
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 1})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("second activation queued %#v, want ActivationLimit$ 1 to stop it",
			e.pendingTriggers)
	}
}
