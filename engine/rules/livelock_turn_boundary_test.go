package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestLivelockWatcherTurnBoundaryBreaksCycle pins the line between a stuck
// resolution and the game taking turns: the same four-event pattern is a
// livelock when it repeats inside a turn (the StepChange control) but a legal
// repeating game once every period begins a fresh turn (the TurnChange case,
// an extra-turn loop whose library holds only the extra-turn card).
func TestLivelockWatcherTurnBoundaryBreaksCycle(t *testing.T) {
	const cycle = 4
	pattern := func(boundary events.Kind) func(int) events.Event {
		return func(i int) events.Event {
			if i%cycle == cycle-1 {
				return events.Event{Kind: boundary, Obj: state.ObjID(4)}
			}
			return events.Event{Kind: events.Note, Obj: state.ObjID(1 + i%cycle)}
		}
	}

	// Precondition: the control pattern is one the detector catches without a
	// turn boundary, so the TurnChange case below is not passing vacuously.
	control := livelockTestEngine(t, &LoopGuard{CycleEvents: 12, MaxPeriod: 8})
	if lle := drivePattern(t, control, pattern(events.Note), 1000); lle == nil {
		t.Fatal("precondition: the control cycle with no turn boundary did not trip the watcher")
	}

	e := livelockTestEngine(t, &LoopGuard{CycleEvents: 12, MaxPeriod: 8})
	if lle := drivePattern(t, e, pattern(events.TurnChange), 1000); lle != nil {
		t.Fatalf("watcher fired on a cycle that begins a fresh turn each period: %v", lle)
	}
}
