package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// sawCostWindow builds Saw attached to a Grizzly Bears with Hill Giant as the
// only other permanent, declares the Bears as an attacker, resolves Saw's
// Attacks trigger, and returns the trigger-cost window with its pay/decline
// option indexes (-1 when absent).
func sawCostWindow(t *testing.T) (e *Engine, saw, bear, giant state.ObjID, pay, decline int) {
	t.Helper()
	e = layerEngine(t)
	saw = onBoardCard(t, e, 0, corpusCard(t, "Saw"))
	bear = onBoardCard(t, e, 0, corpusCard(t, "Grizzly Bears"))
	giant = onBoardCard(t, e, 0, corpusCard(t, "Hill Giant"))
	e.emit(events.Event{Kind: events.Attach, Obj: saw, IDs: []state.ObjID{bear}})
	for _, id := range []state.ObjID{saw, bear, giant} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
			t.Fatalf("precondition: %d = %+v, want battlefield under seat 0", id, o)
		}
	}
	if o := e.G.Obj(saw); o.AttachedTo != bear {
		t.Fatalf("precondition: Saw attached to %d, want the Bears %d", o.AttachedTo, bear)
	}
	// Player in a DeclareAttackers event is the DEFENDING seat.
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear}})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pending triggers after the declaration = %d, want Saw's one", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("Saw's trigger did not open a choose window: %+v", d)
	}
	pay, decline = -1, -1
	for _, o := range d.Options {
		switch o.Kind {
		case "trigger_cost_pay":
			pay = o.Index
		case "trigger_cost_decline":
			decline = o.Index
		}
	}
	if decline < 0 {
		t.Fatalf("the cost window has no decline answer: %+v", d.Options)
	}
	return e, saw, bear, giant, pay, decline
}

func sawEventsSince(e *Engine, from int, pred func(events.Event) bool) int {
	n := 0
	for _, ev := range e.L.Events[from:] {
		if pred(ev) {
			n++
		}
	}
	return n
}

// TestSawNotDefinedTriggeredAttackerCostPays: Saw's "sacrifice a permanent
// other than that creature or Saw" is payable, the attacker is excluded by
// NotDefinedTriggeredAttacker and Saw by Permanent.Other, so the only
// eligible permanent (Hill Giant) is sacrificed and a card is drawn.
func TestSawNotDefinedTriggeredAttackerCostPays(t *testing.T) {
	e, saw, bear, giant, pay, _ := sawCostWindow(t)
	if pay < 0 {
		t.Fatalf("the Sac<1/Permanent.Other+NotDefinedTriggeredAttacker> cost was not offered as payable: %+v", e.Pending().Options)
	}
	libBefore := len(e.G.Zone(state.ZLibrary, 0))
	mark := len(e.L.Events)
	submitChoices(t, e, pay)
	passUntilStackEmpty(t, e, 20)

	var sacrificed []state.ObjID
	for _, ev := range e.L.Events[mark:] {
		if events.IsSacrifice(ev) {
			sacrificed = append(sacrificed, ev.Obj)
		}
	}
	if len(sacrificed) != 1 || sacrificed[0] != giant {
		t.Errorf("sacrificed = %v, want exactly Hill Giant %d", sacrificed, giant)
	}
	if n := sawEventsSince(e, mark, func(ev events.Event) bool { return ev.Kind == events.Draw && ev.Player == 0 }); n != 1 {
		t.Errorf("draw events for seat 0 = %d, want 1", n)
	}
	if got := len(e.G.Zone(state.ZLibrary, 0)); got != libBefore-1 {
		t.Errorf("library = %d, want %d", got, libBefore-1)
	}
	if o := e.G.Obj(giant); o.Zone != state.ZGraveyard {
		t.Errorf("Hill Giant zone = %v, want graveyard", o.Zone)
	}
	if o := e.G.Obj(bear); o.Zone != state.ZBattlefield {
		t.Errorf("the equipped attacker zone = %v, want battlefield", o.Zone)
	}
	if o := e.G.Obj(saw); o.Zone != state.ZBattlefield || o.AttachedTo != bear {
		t.Errorf("Saw zone=%v attachedTo=%d, want battlefield attached to %d", o.Zone, o.AttachedTo, bear)
	}
}

// TestSawNotDefinedTriggeredAttackerCostDeclines: the pay answer is offered
// (precondition) and declining it moves nothing.
func TestSawNotDefinedTriggeredAttackerCostDeclines(t *testing.T) {
	e, saw, bear, giant, pay, decline := sawCostWindow(t)
	if pay < 0 {
		t.Fatalf("the cost window did not offer a pay answer before the decline: %+v", e.Pending().Options)
	}
	libBefore := len(e.G.Zone(state.ZLibrary, 0))
	mark := len(e.L.Events)
	submitChoices(t, e, decline)
	passUntilStackEmpty(t, e, 20)

	if n := sawEventsSince(e, mark, events.IsSacrifice); n != 0 {
		t.Errorf("sacrifice events after declining = %d, want 0", n)
	}
	if n := sawEventsSince(e, mark, func(ev events.Event) bool { return ev.Kind == events.Draw && ev.Player == 0 }); n != 0 {
		t.Errorf("draw events after declining = %d, want 0", n)
	}
	for _, id := range []state.ObjID{saw, bear, giant} {
		if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
			t.Errorf("object %d zone = %v, want battlefield", id, o.Zone)
		}
	}
	if o := e.G.Obj(saw); o.AttachedTo != bear {
		t.Errorf("Saw attachedTo = %d, want %d", o.AttachedTo, bear)
	}
	if got := len(e.G.Zone(state.ZLibrary, 0)); got != libBefore {
		t.Errorf("library = %d, want %d", got, libBefore)
	}
}
