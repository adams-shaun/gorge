package rules

// CR 610.3b for Palace Jailer's "exile target creature an opponent controls
// until an opponent becomes the monarch": when an opponent becomes the
// monarch after the exile trigger went on the stack, the "until" event has
// already happened, so the creature does not move and no return is armed.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// jailerExileOnStack builds the Palace Jailer board (seat 0's Jailer, seat
// 1's Grizzly Bears), puts both ETB triggers on the stack with the exile
// trigger on top, targets the bears, and returns the bears' ID.
func jailerExileOnStack(t *testing.T, e *Engine) state.ObjID {
	t.Helper()
	bf := e.G.Zone(state.ZBattlefield, 1)
	if len(bf) != 1 || e.G.Obj(bf[0]).Face().Name != "Grizzly Bears" {
		t.Fatalf("precondition: seat 1 battlefield = %v, want only Grizzly Bears", bf)
	}
	victim := bf[0]
	e.putTriggersOnStack()
	submitChoices(t, e, 0, 1) // exile trigger on top
	submitChoices(t, e, 0)    // target the bears
	if len(e.G.Stack) != 2 {
		t.Fatalf("precondition: stack = %v, want both Palace Jailer triggers", e.G.Stack)
	}
	top := e.G.Obj(e.G.Stack[len(e.G.Stack)-1])
	if top.Ability == nil || top.Ability.API != "ChangeZone" {
		t.Fatalf("precondition: top of stack is %+v, want the ChangeZone exile trigger", top.Ability)
	}
	return victim
}

func countBecomeMonarchDelayed(e *Engine) int {
	n := 0
	for _, d := range e.G.Delayed {
		if d.EventMode == "BecomeMonarch" {
			n++
		}
	}
	return n
}

// TestPalaceJailerExilesNothingWhenOpponentTookCrownAfterTrigger: seat 1
// becomes the monarch after the exile trigger is on the stack, so the bears
// stay on the battlefield and no return is registered.
func TestPalaceJailerExilesNothingWhenOpponentTookCrownAfterTrigger(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := linkBoard(t, reg, []string{"Palace Jailer"}, []string{"Grizzly Bears"})
	victim := jailerExileOnStack(t, e)
	top := e.G.Obj(e.G.Stack[len(e.G.Stack)-1])

	e.emit(events.Event{Kind: events.MonarchChange, Player: 1})
	if !e.G.IsMonarch(1) || e.G.Players[1].BecameMonarchSeq <= top.PushMonarchSeq {
		t.Fatalf("precondition: seat 1 must take the crown after the push (became %d, push %d)",
			e.G.Players[1].BecameMonarchSeq, top.PushMonarchSeq)
	}
	e.resolveTop()

	if got := e.G.Obj(victim).Zone; got != state.ZBattlefield {
		t.Fatalf("victim zone = %s, want battlefield (CR 610.3b: the opponent already became the monarch)", got)
	}
	if n := countBecomeMonarchDelayed(e); n != 0 {
		t.Fatalf("BecomeMonarch registrations = %d (%+v), want none", n, e.G.Delayed)
	}
}

// TestPalaceJailerExilesWhenOpponentWasMonarchBeforeTrigger is the control:
// seat 1 was already the monarch before the Jailer entered, so no opponent
// became the monarch after the trigger and the bears are exiled with one
// return registered.
func TestPalaceJailerExilesWhenOpponentWasMonarchBeforeTrigger(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := linkBoard(t, reg, []string{"Palace Jailer"}, []string{"Grizzly Bears"})
	e.emit(events.Event{Kind: events.MonarchChange, Player: 1})
	victim := jailerExileOnStack(t, e)
	top := e.G.Obj(e.G.Stack[len(e.G.Stack)-1])
	if !e.G.IsMonarch(1) || e.G.Players[1].BecameMonarchSeq == 0 ||
		e.G.Players[1].BecameMonarchSeq > top.PushMonarchSeq {
		t.Fatalf("precondition: seat 1 must hold the crown from before the push (became %d, push %d)",
			e.G.Players[1].BecameMonarchSeq, top.PushMonarchSeq)
	}
	if got := e.G.Obj(victim).Zone; got != state.ZBattlefield {
		t.Fatalf("precondition: victim zone = %s, want battlefield", got)
	}
	e.resolveTop()

	if got := e.G.Obj(victim).Zone; got != state.ZExile {
		t.Fatalf("victim zone = %s, want exile", got)
	}
	if n := countBecomeMonarchDelayed(e); n != 1 {
		t.Fatalf("BecomeMonarch registrations = %d (%+v), want one", n, e.G.Delayed)
	}
	// The registration jails the exiled bears (not the Jailer the trigger's
	// own memory names), so an opponent taking the crown later returns them.
	e.emit(events.Event{Kind: events.MonarchChange, Player: 0})
	e.emit(events.Event{Kind: events.MonarchChange, Player: 1})
	if got := e.G.Obj(victim).Zone; got != state.ZBattlefield {
		t.Fatalf("victim zone = %s after seat 1 retook the crown, want battlefield", got)
	}
}
