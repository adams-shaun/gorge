package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
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

// CR 725.2's crown steal must survive a PARKED combat hit: when the hit's
// CR 616.1 replacement competition is posed (Battletide Alchemist's optional
// "you may prevent X" alone parks every player hit), the hit's landed event
// in finishChosenDamage queues the steal -- runCombatAssignments' synchronous
// branch is never reached. Both arms of the ask (declining the prevention,
// and applying it) land the hit and hand the crown to the attacker's
// controller.
func TestMonarchStealSurvivesParkedReplacement(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	deck0 := mountainDeck(t, 40)
	deck1 := mountainDeck(t, 41)
	bears, ok := reg.Lookup("Grizzly Bears")
	if !ok {
		t.Fatalf("corpus fixture: Grizzly Bears missing")
	}
	deck0 = append(deck0, bears)
	batt, ok := reg.Lookup("Battletide Alchemist")
	if !ok {
		t.Fatalf("corpus fixture: Battletide Alchemist missing")
	}
	deck1 = append(deck1, batt)
	e := New(seatZeroStart(Config{Seed: 42, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck0, deck1}}))
	e.Advance()
	bear := crAbortMove(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	crAbortMove(t, e, 1, "Battletide Alchemist", state.ZBattlefield)
	for _, tc := range []struct {
		name       string
		choice     int
		wantLanded int32
	}{{"declined prevention lands 2", -1, 2}, {"applied prevention lands 1", 0, 1}} {
		e.emit(events.Event{Kind: events.TurnChange, Player: 1})
		e.pending = nil
		e.emit(events.Event{Kind: events.MonarchChange, Player: 1})
		if !e.G.IsMonarch(1) {
			t.Fatalf("%s: precondition: seat 1 must be monarch", tc.name)
		}
		e.pendingTriggers = nil
		e.combatRound.assignments = []assignment{{toPlayer: 1, amount: 2, from: bear}}
		e.runCombatAssignments()
		d := e.Pending()
		if d == nil || d.Kind != decision.KReplacement || d.Player != 1 {
			t.Fatalf("%s: pending = %+v, want Battletide's optional-prevention ask to seat 1", tc.name, d)
		}
		choice := tc.choice
		if choice < 0 {
			choice = len(d.Options) - 1
		}
		lifeBefore := e.G.Players[1].Life
		submitChoices(t, e, choice)
		e.pending = nil
		if landed := lifeBefore - e.G.Players[1].Life; landed != tc.wantLanded {
			t.Fatalf("%s: landed damage = %d, want %d", tc.name, landed, tc.wantLanded)
		}
		// The parked resolution's own drain already moved the queued steal
		// onto the stack: exactly one source-less ability, controlled by the
		// monarch (CR 725.2), never by the attacker.
		if len(e.G.Stack) != 1 {
			t.Fatalf("%s: stack = %v, want exactly one queued steal ability", tc.name, e.G.Stack)
		}
		o := e.G.Obj(e.G.Stack[0])
		if o.Source != 0 || o.Controller != 1 {
			t.Fatalf("%s: steal ability source %d controller %d, want no source and the monarch's control",
				tc.name, o.Source, o.Controller)
		}
		e.resolveTop()
		if !e.G.IsMonarch(0) {
			t.Fatalf("%s: parked combat hit never stole the crown: monarch=%d", tc.name, e.G.Monarch)
		}
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
