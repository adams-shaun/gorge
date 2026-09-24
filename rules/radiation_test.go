package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// The inherent CR 728.1 ability exists without any card permanent to serve as
// its source. Its event-created stack object must retain that zero source.
func TestRadiationDrainIsSourceLessTriggeredAbility(t *testing.T) {
	e := newSeats(t, 2)
	seedPlayerCounter(t, e, 0, "RAD", 2)
	if got := e.G.Players[0].Counter("RAD"); got != 2 {
		t.Fatalf("precondition: RAD=%d, want 2", got)
	}
	e.G.Step = state.StepMain1
	e.finishEnteredStep()
	if len(e.pendingTriggers) != 1 || !e.pendingTriggers[0].RadiationDrain {
		t.Fatalf("precombat-main did not queue radiation trigger: %+v", e.pendingTriggers)
	}
	e.pushTrigger(e.pendingTriggers[0])
	if len(e.G.Stack) != 1 {
		t.Fatalf("radiation ability stack length=%d, want 1", len(e.G.Stack))
	}
	o := e.G.Obj(e.G.Stack[0])
	if o == nil || o.StackKind != state.StackKindTriggered || o.Source != 0 || o.Controller != 0 {
		t.Fatalf("inherent drain stack object=%+v, want triggered, source-less, controller 0", o)
	}
	if got := e.G.Players[0].Counter("RAD"); got != 2 {
		t.Fatalf("RAD changed before response window: %d, want 2", got)
	}
}

func TestRadiationDrainNotQueuedWithoutCounters(t *testing.T) {
	e := newSeats(t, 2)
	if got := e.G.Players[0].Counter("RAD"); got != 0 {
		t.Fatalf("precondition: RAD=%d, want 0", got)
	}
	e.G.Step = state.StepMain1
	e.finishEnteredStep()
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("zero-RAD player queued a drain: %+v", e.pendingTriggers)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("zero-RAD player has stack object(s): %v", e.G.Stack)
	}
}
