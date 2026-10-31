package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDelayedReturnSkipsTheObjectReturnedWhileItsTriggerWaited pins CR 400.7
// for a dies trigger that resolves after its triggering card came back and
// died again: the trigger refers to the object that died the FIRST time, so
// the delayed "return it at the beginning of the next end step" registration
// it makes must stamp the remembered object as it stood when the trigger
// FIRED, not as it stands when the trigger finally resolves. Nine-Lives
// Familiar died with revival counters; while its dies trigger sat on the
// stack another effect returned it and it died again; when the first trigger
// then resolved, the card in the graveyard was a new object, and the first
// trigger's delayed return must move nothing. dirac K5p pairs 12963
// and 80097. TestDelayedReturnFindsOnlyTheRememberedObject is the sibling:
// there the trigger resolved BEFORE the round trip, so the registration-time
// stamp was already the first graveyard incarnation, and the control arm of
// this shape must keep returning the Familiar.
func TestDelayedReturnSkipsTheObjectReturnedWhileItsTriggerWaited(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Nine-Lives Familiar"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MB] = 1, 2
	castMode(t, e, id, "")
	finishCast(t, e, id)
	if got := e.G.Obj(id).Counter("REVIVAL"); got != 8 {
		t.Fatalf("precondition: entered with %d revival counters, want 8", got)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	deathIncarnation := e.G.Obj(id).Incarnation
	if deathIncarnation == 0 {
		t.Fatalf("precondition: leaving the battlefield did not advance the incarnation")
	}
	// Park the dies trigger on the stack, unresolved.
	if n := len(e.pendingTriggers); n != 1 {
		t.Fatalf("first death queued %d triggers, want the Familiar's 1", n)
	}
	e.putTriggersOnStack()
	if e.Pending() != nil {
		t.Fatalf("precondition: placing the parked trigger asked %v", e.Pending().Kind)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: %d objects on the stack, want the parked trigger's 1", len(e.G.Stack))
	}
	// Another effect returns the card while the trigger waits. Leaving the
	// battlefield clears its counters, so the second death fires nothing.
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZBattlefield})
	if got := e.G.Obj(id).Counter("REVIVAL"); got != 0 {
		t.Fatalf("precondition: returned with %d revival counters, want 0", got)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	if e.G.Obj(id).Incarnation <= deathIncarnation {
		t.Fatalf("precondition: second death did not mint a new object (incarnation %d)", e.G.Obj(id).Incarnation)
	}
	if n := len(e.pendingTriggers); n != 0 {
		t.Fatalf("second death queued %d triggers, want 0 (the card returned without revival counters)", n)
	}
	// The FIRST trigger finally resolves, out of round.
	drainQueuedTriggers(t, e)
	if len(e.G.Delayed) != 1 {
		t.Fatalf("the resolving trigger registered %d delayed triggers, want 1", len(e.G.Delayed))
	}
	passUntil(t, e, func() bool {
		return len(e.G.Delayed) == 0 && len(e.G.Stack) == 0 && e.Pending() != nil
	})
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("first trigger's delayed return moved the new object: zone %v, want graveyard", z)
	}
}
