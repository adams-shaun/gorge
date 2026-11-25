package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// suspendTriggerOnStack returns the id of the triggered ability waiting on
// the stack whose source is src and whose body API is api, or 0.
func suspendTriggerOnStack(e *Engine, src state.ObjID, api string) state.ObjID {
	for _, sid := range e.G.Stack {
		so := e.G.Obj(sid)
		if so != nil && so.Ability != nil && so.Source == src && so.Ability.API == api {
			return sid
		}
	}
	return 0
}

// suspendTriggerWaiting reports whether src's suspend tick or may-cast
// trigger is still waiting: on the stack, or queued for the next placement.
func suspendTriggerWaiting(e *Engine, src state.ObjID) bool {
	if suspendTriggerOnStack(e, src, "SuspendTick") != 0 ||
		suspendTriggerOnStack(e, src, "SuspendCast") != 0 {
		return true
	}
	for _, pt := range e.pendingTriggers {
		if (pt.SuspendTick || pt.SuspendLast) && pt.Source == src {
			return true
		}
	}
	return false
}

// suspendResolveTick passes the outstanding priority rounds until src's
// suspend triggers — the tick waiting on the stack, and any may-cast trigger
// the tick queued — have all resolved, stopping at the first non-priority ask
// (a may-cast choice) or at the fresh priority round after the last
// resolution. It leaves the engine wherever the passes ended.
func suspendResolveTick(t *testing.T, e *Engine, src state.ObjID) {
	t.Helper()
	for i := 0; i < 500; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			d = e.Pending()
			if d == nil || e.G.Over {
				return
			}
		}
		if d.Kind != decision.KPriority {
			return
		}
		if !suspendTriggerWaiting(e, src) {
			return
		}
		passPriorityAll(t, e)
	}
}

// TestSuspendUpkeepTriggerUsesTheStack pins CR 702.62a/b as real triggered
// abilities (CR 603.3): a suspended card's owner's upkeep puts the
// "remove a time counter" trigger on the stack as a respondable ability, the
// TIME counter leaves only when it resolves, and players receive priority
// with it there. The turn-based decrement this replaces never touched the
// stack and never gave anyone a window.
func TestSuspendUpkeepTriggerUsesTheStack(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, "Name:Filler\nManaCost:3 R\nTypes:Sorcery\nOracle:x\n"))
	o := e.G.AddObject(card(t, "Name:Bloom\nManaCost:0\nTypes:Artifact\nK:Suspend:3:U\nOracle:x\n"), 0)
	o.Zone = state.ZExile
	e.G.SetZone(state.ZExile, 0, []state.ObjID{o.ID})
	e.emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: "TIME", Amount: 3})
	// Precondition the assertions depend on: the fixture card is suspended in
	// its owner's exile pile with three time counters.
	if got := e.G.Obj(o.ID); got.Zone != state.ZExile || got.Counter("TIME") != 3 || !hasSuspendAbility(got) {
		t.Fatalf("precondition: suspended fixture = zone %s TIME %d suspend %v",
			got.Zone, got.Counter("TIME"), hasSuspendAbility(got))
	}

	e.beginTurn(0)
	e.Advance()
	// The trigger is on the stack and the counter is still there.
	if trig := suspendTriggerOnStack(e, o.ID, "SuspendTick"); trig == 0 {
		t.Fatal("no suspend tick trigger on the stack at the upkeep")
	}
	if got := e.G.Obj(o.ID).Counter("TIME"); got != 3 {
		t.Fatalf("TIME with the trigger unresolved = %d, want 3", got)
	}
	// Priority is asked with the trigger there; the active player passes and
	// the opponent gets it too, the trigger still unresolved.
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("pending = %+v, want priority with the suspend trigger on the stack", d)
	}
	submitChoices(t, e, passPriorityOption(t, d))
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.Player != 1 {
		t.Fatalf("pending = %+v, want the opponent's priority, trigger still waiting", d)
	}
	if got := e.G.Obj(o.ID).Counter("TIME"); got != 3 {
		t.Fatalf("TIME while the trigger waits = %d, want 3", got)
	}
	// The second pass resolves the trigger; exactly one counter leaves.
	submitChoices(t, e, passPriorityOption(t, e.Pending()))
	if got := e.G.Obj(o.ID).Counter("TIME"); got != 2 {
		t.Fatalf("TIME after the trigger resolved = %d, want 2", got)
	}
	if trig := suspendTriggerOnStack(e, o.ID, "SuspendTick"); trig != 0 {
		t.Fatal("the resolved tick trigger is still on the stack")
	}
}

// TestSuspendLastCounterTriggersTheCastOffer pins CR 702.62a's may-cast as
// the resolution of a second triggered ability: the tick that removes the
// last time counter queues it, players receive priority with it on the
// stack, and its resolution asks the owner — a decline leaves the card in
// exile.
func TestSuspendLastCounterTriggersTheCastOffer(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, "Name:Filler\nManaCost:3 R\nTypes:Sorcery\nOracle:x\n"))
	o := e.G.AddObject(card(t, "Name:Gargadon\nManaCost:7 R\nTypes:Creature Elephant\nPT:9/7\nK:Suspend:1:R\nOracle:x\n"), 0)
	o.Zone = state.ZExile
	e.G.SetZone(state.ZExile, 0, []state.ObjID{o.ID})
	e.emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: "TIME", Amount: 1})

	e.beginTurn(0)
	e.Advance()
	// The tick is on the stack with the counter still on the card.
	if trig := suspendTriggerOnStack(e, o.ID, "SuspendTick"); trig == 0 {
		t.Fatal("no suspend tick trigger on the stack at the upkeep")
	}
	if got := e.G.Obj(o.ID).Counter("TIME"); got != 1 {
		t.Fatalf("TIME with the tick unresolved = %d, want 1", got)
	}
	// Both players pass; the tick resolves and removes the LAST counter,
	// which queues the may-cast trigger in its turn.
	submitChoices(t, e, passPriorityOption(t, e.Pending()))
	submitChoices(t, e, passPriorityOption(t, e.Pending()))
	if got := e.G.Obj(o.ID).Counter("TIME"); got != 0 {
		t.Fatalf("TIME after the last-counter tick = %d, want 0", got)
	}
	cast := suspendTriggerOnStack(e, o.ID, "SuspendCast")
	if cast == 0 {
		t.Fatal("no may-cast trigger on the stack after the last counter left")
	}
	// Players receive priority with it there before it resolves.
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("pending = %+v, want priority with the may-cast trigger on the stack", d)
	}
	submitChoices(t, e, passPriorityOption(t, d))
	submitChoices(t, e, passPriorityOption(t, e.Pending()))
	// The may-cast trigger's resolution asks the owner.
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 || d.Options[0].Kind != "suspend_cast_yes" {
		t.Fatalf("pending = %+v, want the may-cast ask", d)
	}
	// A decline leaves the card in exile.
	submitChoices(t, e, 1)
	if got := e.G.Obj(o.ID); got.Zone != state.ZExile {
		t.Fatalf("declined suspend cast moved the card to %s, want exile", got.Zone)
	}
}
