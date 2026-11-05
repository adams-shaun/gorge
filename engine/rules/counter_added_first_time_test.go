package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// firstTimeCounterProbe is a synthetic CounterAddedOnce line with
// FirstTime$ True, the shape of U.S.S. Enterprise-D's charge-counter trigger
// with a harmless body.
const firstTimeCounterProbe = "Name:First-Time Counter Probe\nTypes:Artifact\n" +
	"T:Mode$ CounterAddedOnce | ValidCard$ Card.Self | TriggerZones$ Battlefield | CounterType$ CHARGE | FirstTime$ True | Execute$ TrigFirstTimeProbe | TriggerDescription$ Whenever one or more charge counters are put on CARDNAME for the first time each turn, CARDNAME gets +2/+2 until end of turn.\n" +
	"SVar:TrigFirstTimeProbe:DB$ Pump | Defined$ Self | NumAtt$ +2 | NumDef$ +2\nOracle:x\n"

// TestCounterAddedOnceFirstTimeEachTurn pins FirstTime$ True on a
// CounterAddedOnce line: only the turn's first matching CounterChange batch
// queues the trigger. The counter kind is part of the gate -- another kind's
// batch neither fires the line nor spends its first-time latch -- and a
// TurnChange opens the next turn's first batch.
func TestCounterAddedOnceFirstTimeEachTurn(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	id := onBoard(t, e, 0, firstTimeCounterProbe)

	// Another counter kind is a different placement event: it does not fire
	// the line and must not spend its first-time latch.
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 1})
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("pendingTriggers after a P1P1 batch = %d, want 0", len(e.pendingTriggers))
	}

	// The turn's first CHARGE batch fires, however many counters it carries.
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "CHARGE", Amount: 3})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pendingTriggers after the first CHARGE batch = %d, want 1", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()

	// A later batch in the same turn is not "the first time".
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "CHARGE", Amount: 1})
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("pendingTriggers after a second CHARGE batch = %d, want 0", len(e.pendingTriggers))
	}

	// A new turn opens a new first CHARGE batch.
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: e.G.Turn + 1})
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "CHARGE", Amount: 1})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pendingTriggers after the next turn's first CHARGE batch = %d, want 1", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()

	// ...and its second batch is refused like the previous turn's.
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "CHARGE", Amount: 2})
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("pendingTriggers after the next turn's second CHARGE batch = %d, want 0", len(e.pendingTriggers))
	}
}
