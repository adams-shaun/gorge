package rules

// A delayed triggered ability whose body names Defined$ Self must not follow
// its source once the source has left the battlefield and returned: the card
// that comes back is a new object (CR 400.7, 603.7c). Fortune, Loyal Steed's
// second end-of-combat trigger is the corpus case; this file uses an inline
// fixture with the same shape (register at end of turn, then ChangeZone Self
// to exile and back).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// delayedSelfReturnSrc registers, from its own enters trigger, a delayed
// trigger whose body blinks Defined$ Self.
const delayedSelfReturnSrc = "Name:Delayed Self Return Test\nManaCost:1\nTypes:Enchantment\n" +
	"T:Mode$ ChangesZone | Origin$ Hand | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigRegister\n" +
	"SVar:TrigRegister:DB$ DelayedTrigger | Mode$ Phase | Phase$ End of Turn | Execute$ TrigExile\n" +
	"SVar:TrigExile:DB$ ChangeZone | Origin$ Battlefield | Defined$ Self | Destination$ Exile | SubAbility$ DBReturn\n" +
	"SVar:DBReturn:DB$ ChangeZone | Defined$ Self | Origin$ Exile | Destination$ Battlefield\n" +
	"Oracle:x\n"

// delayedSelfFixture enters the fixture and resolves its enters trigger, so
// one DelayedTrigger registration waits for the end step. It returns the
// engine and the fixture's id.
func delayedSelfFixture(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	o := e.G.AddObject(card(t, delayedSelfReturnSrc), 0)
	o.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), o.ID))
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZHand, To: state.ZBattlefield})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack = %d, want the one enters trigger", len(e.G.Stack))
	}
	e.resolveTop()
	if len(e.G.Delayed) != 1 {
		t.Fatalf("precondition: delayed registrations = %d, want one", len(e.G.Delayed))
	}
	if got := e.G.Obj(o.ID); got == nil || got.Zone != state.ZBattlefield {
		t.Fatalf("precondition: fixture zone = %v, want battlefield", got)
	}
	return e, o.ID
}

// movesSince counts the seeded object's MoveZone events recorded after mark.
func movesSince(e *Engine, mark int, id state.ObjID) int {
	n := 0
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.MoveZone && ev.Obj == id {
			n++
		}
	}
	return n
}

// TestDelayedSelfSkipsReturnedSource: the source crossed the battlefield
// boundary before the delayed trigger resolved, so the body's Defined$ Self
// names a new object and the blink does nothing.
func TestDelayedSelfSkipsReturnedSource(t *testing.T) {
	t.Parallel()
	e, src := delayedSelfFixture(t)
	before := e.G.Obj(src).Incarnation

	// Round-trip the source through exile and back: a new battlefield object.
	e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZBattlefield, To: state.ZExile})
	e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZExile, To: state.ZBattlefield})
	o := e.G.Obj(src)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: source zone = %v, want battlefield after the round trip", o)
	}
	if o.Incarnation == before {
		t.Fatalf("precondition: incarnation = %d before and after the round trip, want a new one", before)
	}
	if len(e.G.Delayed) != 1 {
		t.Fatalf("precondition: delayed registrations = %d after the round trip, want one", len(e.G.Delayed))
	}

	mark := len(e.L.Events)
	driveToEndStep(t, e)
	for e.putTriggersOnStack() {
		answerTriggerOrders(t, e)
		if len(e.G.Stack) != 0 {
			break
		}
	}
	if len(e.G.Stack) == 0 {
		t.Fatal("the delayed trigger never reached the stack")
	}
	passUntilStackEmpty(t, e, 40)
	if n := movesSince(e, mark, src); n != 0 {
		t.Fatalf("returned source moved %d times in the delayed resolution, want none (CR 603.7c)", n)
	}
	if got := e.G.Obj(src); got == nil || got.Zone != state.ZBattlefield {
		t.Fatalf("returned source zone = %v after the delayed trigger, want battlefield", got)
	}
}

// TestDelayedSelfBlinkStaysLive is the control: a source that never left the
// battlefield is still the object the delayed body names, so the blink moves
// it to exile and back.
func TestDelayedSelfBlinkStaysLive(t *testing.T) {
	t.Parallel()
	e, src := delayedSelfFixture(t)
	if got := e.G.Obj(src).Zone; got != state.ZBattlefield {
		t.Fatalf("precondition: source zone = %v, want battlefield", got)
	}
	mark := len(e.L.Events)
	driveToEndStep(t, e)
	for e.putTriggersOnStack() {
		answerTriggerOrders(t, e)
		if len(e.G.Stack) != 0 {
			break
		}
	}
	if len(e.G.Stack) == 0 {
		t.Fatal("the delayed trigger never reached the stack")
	}
	passUntilStackEmpty(t, e, 40)
	if n := movesSince(e, mark, src); n != 2 {
		t.Fatalf("live source moved %d times in the delayed resolution, want the exile and return pair", n)
	}
	if got := e.G.Obj(src); got == nil || got.Zone != state.ZBattlefield {
		t.Fatalf("live source zone = %v after the delayed trigger, want battlefield", got)
	}
}
