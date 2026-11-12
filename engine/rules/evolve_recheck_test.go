package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEvolveRechecksItsConditionOnResolution pins CR 702.100a / 603.4: evolve's
// "if that creature's power is greater ... and/or its toughness is greater" is
// an intervening if, checked as the trigger fires AND again as it resolves.
// Two 2/2 creatures entering together each queue an evolve on a 1/1 Experiment
// One; the first resolves and makes it a 2/2, so the second no longer finds a
// greater power or toughness and does nothing. Checked only as it fired, the
// engine put two counters.
func TestEvolveRechecksItsConditionOnResolution(t *testing.T) {
	t.Parallel()
	const evolver, bears = "Experiment One", "Grizzly Bears"
	e, cfg, _ := evolvedGame(t, 91, evolver, bears, bears)

	src := enterFromLibrary(t, e, 0, evolver)
	e.priorityRound()
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(src); o == nil || o.Zone != state.ZBattlefield || o.Counter("P1P1") != 0 {
		t.Fatalf("precondition: Experiment One must sit on the battlefield with no counters: %+v", o)
	}
	if p, tt := e.Power(src), e.Toughness(src); p != 1 || tt != 1 {
		t.Fatalf("precondition: Experiment One is %d/%d, want 1/1", p, tt)
	}

	b1 := enterFromLibrary(t, e, 0, bears)
	b2 := enterFromLibrary(t, e, 0, bears)
	for _, b := range []state.ObjID{b1, b2} {
		if p, tt := e.Power(b), e.Toughness(b); p != 2 || tt != 2 {
			t.Fatalf("precondition: entrant is %d/%d, want 2/2", p, tt)
		}
	}
	if got := len(e.pendingTriggers); got != 2 {
		t.Fatalf("precondition: %d pending evolve triggers, want 2 (one per entrant)", got)
	}
	e.priorityRound()
	answerTriggerOrders(t, e)
	passUntilStackEmpty(t, e, 40)

	if got := e.G.Obj(src).Counter("P1P1"); got != 1 {
		t.Fatalf("Experiment One +1/+1 counters = %d after two 2/2s entered together, want 1 (the second evolve's recheck fails)", got)
	}
	if n := evolvedMarkerCount(e, src); n != 1 {
		t.Fatalf("events.Evolved = %d, want 1", n)
	}
	replayCheck(t, e, cfg)
}

// TestEvolveCountsATokenEntering pins that a token entering is a creature
// entering (CR 702.100a, 111.1): a 1/1 Soldier token evolves a 0/3 Dinosaur
// Egg, and two of them minted together evolve it once, the second token's
// comparison failing as the second trigger resolves. The token's mint event
// carries no move zones, which the Evolve$ gate once read as "not entering".
func TestEvolveCountsATokenEntering(t *testing.T) {
	t.Parallel()
	const egg = "Dinosaur Egg"
	e, cfg, _ := conditionEvolveGame(t, 92, egg)

	src := enterFromLibrary(t, e, 0, egg)
	e.priorityRound()
	passUntilStackEmpty(t, e, 40)
	if p, tt := e.Power(src), e.Toughness(src); p != 0 || tt != 3 {
		t.Fatalf("precondition: Dinosaur Egg is %d/%d, want 0/3", p, tt)
	}

	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "w_1_1_soldier"})
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "w_1_1_soldier"})
	if got := len(e.pendingTriggers); got != 2 {
		t.Fatalf("precondition: %d pending evolve triggers after two 1/1 tokens, want 2", got)
	}
	e.priorityRound()
	answerTriggerOrders(t, e)
	passUntilStackEmpty(t, e, 40)

	if got := e.G.Obj(src).Counter("P1P1"); got != 1 {
		t.Fatalf("Dinosaur Egg +1/+1 counters = %d after two 1/1 tokens, want 1", got)
	}
	replayCheck(t, e, cfg)
}
