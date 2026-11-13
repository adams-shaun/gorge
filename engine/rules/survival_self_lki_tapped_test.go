package rules

// The CR 603.4 intervening-if recheck of a PresentDefined$ Self clause whose
// source left the battlefield reads the source's LAST KNOWN information (CR
// 608.2h), not the live graveyard card: House Cartographer is tapped (it
// attacked), its Survival trigger waits on the stack, and Bear Trap's damage
// kills it in response. Forge and XMage still resolve the trigger and put a
// land into its controller's hand; a live-zone recheck fizzles it.
// presentClauseHolds answers from the departure snapshot bound by
// bindSourceLKI (SourceTapped).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// survivalStandup seats the repro: House Cartographer on seat 0's
// battlefield, tapped, and its Survival trigger fired by the second main
// phase of the same turn. The tests below assert both preconditions their
// recheck depends on: the tapped battlefield creature the clause held for at
// fire time, and the departure the resolution recheck must read past.
func survivalStandup(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	e := crResolutionEngine(t, []string{"House Cartographer", "Bear Trap"}, nil)
	id := crAbortMove(t, e, 0, "House Cartographer", state.ZBattlefield)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: House Cartographer not on the battlefield")
	}
	e.emit(events.Event{Kind: events.Tap, Obj: id})
	if o := e.G.Obj(id); !o.Tapped {
		t.Fatalf("precondition: House Cartographer not tapped after the Tap event")
	}
	passUntilStep(t, e, e.G.Turn, state.StepMain2)
	// The Survival trigger queued against the LIVE tapped creature; give it
	// its stack object.
	for n := 0; n < 100 && !survivalOnStack(e, id); n++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			break
		}
		passOnce(t, e)
	}
	if !survivalOnStack(e, id) {
		t.Fatalf("precondition: Survival trigger not on the stack at second main phase")
	}
	return e, id
}

// survivalOnStack reports whether the Cartographer's Survival ability object
// is waiting on the stack.
func survivalOnStack(e *Engine, id state.ObjID) bool {
	for _, sid := range e.G.Stack {
		if o := e.G.Obj(sid); o != nil && o.Source == id {
			return true
		}
	}
	return false
}

// survivalKillInResponse deals lethal damage and resolves the state-based
// action, putting the Cartographer in its graveyard while the trigger waits.
func survivalKillInResponse(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 9})
	e.checkStateBased()
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: House Cartographer not in the graveyard after lethal damage")
	}
}

// survivalDrain passes until the stack is empty.
func survivalDrain(t *testing.T, e *Engine) {
	t.Helper()
	for n := 0; n < 4000 && !e.G.Over && len(e.G.Stack) > 0; n++ {
		d := e.Pending()
		if d == nil {
			continue
		}
		if d.Kind == decision.KPriority {
			passOnce(t, e)
			continue
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	if len(e.G.Stack) > 0 {
		t.Fatalf("stack never emptied (%d objects left)", len(e.G.Stack))
	}
}

// TestSurvivalResolvesFromLastKnownTappedState (CR 603.4, 608.2h): the
// recheck of "if House Cartographer is tapped" reads the departed source's
// last known tapped status, so the Survival dig still puts a land into the
// controller's hand.
func TestSurvivalResolvesFromLastKnownTappedState(t *testing.T) {
	t.Parallel()
	e, id := survivalStandup(t)
	survivalKillInResponse(t, e, id)
	handBefore := len(e.G.Zone(state.ZHand, 0))
	survivalDrain(t, e)
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore+1 {
		t.Fatalf("Survival after the source died in response: hand %d -> %d, want %d", handBefore, got, handBefore+1)
	}
	if n := interveningIfFizzles(e); n != 0 {
		t.Fatalf("Survival fizzled %d time(s); CR 608.2h keeps the recheck on the last known tapped state", n)
	}
}

// TestSurvivalUntappedSourceStillFizzlesAfterLeaving: the snapshot is the
// status at departure. A Cartographer untapped after its trigger queued died
// untapped, so the recheck's last known information fails the clause and the
// ability is removed; the hand is unchanged.
func TestSurvivalUntappedSourceStillFizzlesAfterLeaving(t *testing.T) {
	t.Parallel()
	e, id := survivalStandup(t)
	e.emit(events.Event{Kind: events.Untap, Obj: id})
	if o := e.G.Obj(id); o.Tapped {
		t.Fatalf("precondition: House Cartographer still tapped after the Untap event")
	}
	survivalKillInResponse(t, e, id)
	handBefore := len(e.G.Zone(state.ZHand, 0))
	survivalDrain(t, e)
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore {
		t.Fatalf("Survival of an untapped departed source: hand %d -> %d, want unchanged", handBefore, got)
	}
	if n := interveningIfFizzles(e); n != 1 {
		t.Fatalf("untapped departed source: %d intervening-if fizzles, want 1", n)
	}
}
