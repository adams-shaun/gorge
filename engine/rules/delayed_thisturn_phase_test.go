package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A Mode$ Phase delayed trigger with ThisTurn$ True and no defined player
// (The Wrath of Khan's chapter I, "at the beginning of the next combat this
// turn") is bounded to the turn it was registered in (CR 603.7): one made
// before the turn's beginning of combat fires there; one made after it never
// fires in a later turn.

func khanDelayedRegistration(t *testing.T, seed uint64, step state.Step) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	khan := mustCorpusCard(t, reg, "The Wrath of Khan")
	e, _ := tokenReplGame(t, seed, khan)
	id := moveSeededCard(t, e, 0, khan, state.ZBattlefield)
	e.Advance()
	driveToStepAll(t, e, 1, 0, step)
	resolveDelayedTriggerAbility(t, e, id, 0, "DBDelTrig")
	return e, id
}

func countGainControlPushes(e *Engine) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.DelayedPush && ev.Counter == "TrigGainControl" {
			n++
		}
	}
	return n
}

func TestDelayedThisTurnPhaseBoundsRegistrationToTheTurn(t *testing.T) {
	t.Parallel()
	e, _ := khanDelayedRegistration(t, 431, state.StepMain2)
	if len(e.G.Delayed) != 1 {
		t.Fatalf("registrations = %d, want 1", len(e.G.Delayed))
	}
	if dt := e.G.Delayed[0]; dt.Phase != state.StepBeginCombat || dt.MaxTurn != e.G.Turn {
		t.Fatalf("registration = %+v, want Phase BeginCombat bounded to turn %d", dt, e.G.Turn)
	}
	// the next turn's beginning of combat (the opponent's, then ours) is past
	// the bound: nothing is pushed
	driveToStepAll(t, e, 3, 0, state.StepMain2)
	if n := countGainControlPushes(e); n != 0 {
		t.Fatalf("the this-turn delayed trigger fired %d time(s) in a later turn", n)
	}
	if len(e.G.Delayed) != 0 {
		t.Fatalf("the expired registration was not collected: %+v", e.G.Delayed)
	}
}

func TestDelayedThisTurnPhaseFiresAtTheNextCombatThisTurn(t *testing.T) {
	t.Parallel()
	e, _ := khanDelayedRegistration(t, 432, state.StepMain1)
	if len(e.G.Delayed) != 1 || e.G.Delayed[0].MaxTurn != e.G.Turn {
		t.Fatalf("registrations = %+v, want one bounded to turn %d", e.G.Delayed, e.G.Turn)
	}
	driveToStepAll(t, e, 1, 0, state.StepBeginCombat)
	e.Advance()
	if n := countGainControlPushes(e); n != 1 {
		t.Fatalf("pushes at the beginning of combat = %d, want 1", n)
	}
}
