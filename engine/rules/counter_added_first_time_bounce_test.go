package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCounterAddedOnceFirstTimeAfterBounce pins CR 400.7 on a FirstTime$ True
// CounterAddedOnce line: a permanent bounced to hand and returned in the same
// turn is a new object, so the first counters put on it after its return are
// "the first time each turn" even though an earlier incarnation took counters
// this turn (U.S.S. Enterprise-D bounced and recast, trk pair 3943). The log
// keeps one ObjID across the trip, so the latch scan stops at the departure.
func TestCounterAddedOnceFirstTimeAfterBounce(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	id := onBoard(t, e, 0, firstTimeCounterProbe)

	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "CHARGE", Amount: 1})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pendingTriggers after the first CHARGE batch = %d, want 1", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()

	// Precondition: a second batch on the same incarnation is refused.
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "CHARGE", Amount: 1})
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("pendingTriggers after a same-incarnation second batch = %d, want 0", len(e.pendingTriggers))
	}

	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZHand})
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("the probe did not return to the battlefield: %+v", o)
	}

	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "CHARGE", Amount: 3})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pendingTriggers after the recast incarnation's first batch = %d, want 1", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()

	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "CHARGE", Amount: 1})
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("pendingTriggers after the recast incarnation's second batch = %d, want 0", len(e.pendingTriggers))
	}
}
