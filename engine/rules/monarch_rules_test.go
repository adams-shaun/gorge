package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestMonarchEndStepDrawUsesTheStack(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	e.emit(events.Event{Kind: events.MonarchChange, Player: 0})
	before := len(e.G.Zone(state.ZHand, 0))
	e.G.Step = state.StepEnd
	e.finishEnteredStep()
	if len(e.pendingTriggers) != 1 || !e.pendingTriggers[0].MonarchDraw {
		t.Fatalf("end step queued %+v, want monarch draw trigger", e.pendingTriggers)
	}
	if len(e.G.Zone(state.ZHand, 0)) != before {
		t.Fatal("monarch draw happened before its triggered ability resolved")
	}
	if e.putTriggersOnStack() || len(e.G.Stack) != 1 {
		t.Fatalf("monarch draw was not put on stack: pending=%+v stack=%v", e.Pending(), e.G.Stack)
	}
	e.resolveTop()
	if got := len(e.G.Zone(state.ZHand, 0)); got != before+1 {
		t.Fatalf("monarch hand size = %d, want %d", got, before+1)
	}
}

func TestCombatDamageToMonarchTransfersTheCrown(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	attacker := onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Grizzly Bears"))
	e.emit(events.Event{Kind: events.MonarchChange, Player: 0})
	if !e.G.IsMonarch(0) || e.G.Obj(attacker).Controller != 1 {
		t.Fatal("precondition: seat 0 must be monarch and seat 1 must control attacker")
	}
	e.combatRound.assignments = []assignment{{from: attacker, toPlayer: 0, amount: 2}}
	e.runCombatAssignments()
	if !e.G.IsMonarch(0) {
		t.Fatal("the crown moved before the triggered ability resolved (CR 725.2)")
	}
	if e.putTriggersOnStack() || len(e.G.Stack) != 1 {
		t.Fatalf("no ability on the stack: pending=%+v stack=%v", e.Pending(), e.G.Stack)
	}
	o := e.G.Obj(e.G.Stack[0])
	if o.Source != 0 || o.Controller != 0 {
		t.Fatalf("source %d controller %d, want no source and the monarch's control (CR 725.2)", o.Source, o.Controller)
	}
	e.resolveTop()
	if !e.G.IsMonarch(1) {
		t.Fatalf("combat damage did not transfer crown: monarch=%d", e.G.Monarch)
	}
}

// Two creatures hitting the monarch are two triggers, each the monarch's; the
// designation moves when they resolve.
func TestEachCreatureHittingTheMonarchTriggers(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	bears := mustCorpusCard(t, reg, "Grizzly Bears")
	a, b := onBoardCard(t, e, 1, bears), onBoardCard(t, e, 1, bears)
	e.emit(events.Event{Kind: events.MonarchChange, Player: 0})
	e.combatRound.assignments = []assignment{{from: a, toPlayer: 0, amount: 2}, {from: b, toPlayer: 0, amount: 2}}
	e.runCombatAssignments()
	if len(e.pendingTriggers) != 2 || !e.pendingTriggers[0].MonarchSteal || e.pendingTriggers[1].MonarchTo != 1 {
		t.Fatalf("pending = %+v, want two monarch triggers", e.pendingTriggers)
	}
}

// "The monarch's end step": another player's end step draws nothing (CR 725.2).
func TestMonarchDrawsOnlyOnTheirOwnEndStep(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	e.emit(events.Event{Kind: events.MonarchChange, Player: 1})
	e.G.Step = state.StepEnd
	e.finishEnteredStep()
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("seat 0's end step queued %+v for monarch seat 1", e.pendingTriggers)
	}
}

// CR 725.4: the active player inherits the crown, or the next seat in turn order
// when the active player is the one leaving.
func TestMonarchLeavingTheGameHandsTheCrownOn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	m := mustCorpusCard(t, reg, "Mountain")
	deck := make([]*cards.Card, 40)
	for i := range deck {
		deck[i] = m
	}
	for _, tc := range []struct {
		name          string
		monarch, want state.PlayerID
	}{{"another seat leaves: the active player", 2, 0}, {"the active player leaves: the next seat", 0, 1}} {
		e := New(seatZeroStart(Config{Seed: 3, Names: []string{"a", "b", "c"}, Tokens: reg.Tokens, Decks: [][]*cards.Card{deck, deck, deck}}))
		e.Advance()
		e.emit(events.Event{Kind: events.MonarchChange, Player: tc.monarch})
		e.emit(events.Event{Kind: events.PlayerLost, Player: tc.monarch})
		e.monarchHandoffOnDeparture(tc.monarch)
		if !e.G.IsMonarch(tc.want) {
			t.Fatalf("%s: monarch = %d, want %d", tc.name, e.G.Monarch, tc.want)
		}
	}
}
