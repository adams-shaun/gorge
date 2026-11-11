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
// boundary after the delayed trigger went on the stack (Fortune, Loyal Steed's
// first trigger blinks it ahead of the second), so the body's Defined$ Self
// names a new object and the blink does nothing. The referent is the source
// as it stood when the trigger fired, the same reference point dirac's
// delayed program takes.
func TestDelayedSelfSkipsReturnedSource(t *testing.T) {
	t.Parallel()
	e, src := delayedSelfFixture(t)
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
	before := e.G.Obj(src).Incarnation

	// Round-trip the source through exile and back while the trigger waits.
	e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZBattlefield, To: state.ZExile})
	e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZExile, To: state.ZBattlefield})
	o := e.G.Obj(src)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: source zone = %v, want battlefield after the round trip", o)
	}
	if o.Incarnation == before {
		t.Fatalf("precondition: incarnation = %d before and after the round trip, want a new one", before)
	}

	mark := len(e.L.Events)
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

// delayedSelfGraveSrc registers a delayed trigger whose body returns
// Defined$ Self from the graveyard (the Ghoulish Impetus / Prized Amalgam
// shape).
const delayedSelfGraveSrc = "Name:Delayed Self Grave Test\nManaCost:1\nTypes:Enchantment\n" +
	"T:Mode$ ChangesZone | Origin$ Hand | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigRegister\n" +
	"SVar:TrigRegister:DB$ DelayedTrigger | Mode$ Phase | Phase$ End of Turn | Execute$ TrigReturn\n" +
	"SVar:TrigReturn:DB$ ChangeZone | Origin$ Graveyard | Defined$ Self | Destination$ Battlefield\n" +
	"Oracle:x\n"

// delayedGraveMoves registers the fixture, puts the source in the graveyard,
// pushes the end-step delayed trigger, optionally bounces the source
// graveyard -> exile -> graveyard while the trigger is on the stack, resolves
// it and counts the source's moves.
func delayedGraveMoves(t *testing.T, roundTrip bool) (*Engine, state.ObjID, int) {
	t.Helper()
	e := layerEngine(t)
	o := e.G.AddObject(card(t, delayedSelfGraveSrc), 0)
	o.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), o.ID))
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZHand, To: state.ZBattlefield})
	e.putTriggersOnStack()
	e.resolveTop()
	if len(e.G.Delayed) != 1 {
		t.Fatalf("precondition: delayed registrations = %d, want one", len(e.G.Delayed))
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZBattlefield, To: state.ZGraveyard})
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
	if got := e.G.Obj(o.ID); got.Zone != state.ZGraveyard {
		t.Fatalf("precondition: source zone = %v at the push, want graveyard", got.Zone)
	}
	if roundTrip {
		gen := e.G.Obj(o.ID).ZoneGen
		e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZGraveyard, To: state.ZExile})
		e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZExile, To: state.ZGraveyard})
		if got := e.G.Obj(o.ID); got.Zone != state.ZGraveyard || got.ZoneGen == gen {
			t.Fatalf("precondition: zone %v gen %d (was %d), want graveyard and a new generation", got.Zone, got.ZoneGen, gen)
		}
	}
	mark := len(e.L.Events)
	passUntilStackEmpty(t, e, 40)
	return e, o.ID, movesSince(e, mark, o.ID)
}

// TestDelayedSelfGraveyardSkipsReturnedSource: a delayed Self from the
// graveyard names the card that was there when the trigger was pushed; one
// that left the graveyard and re-entered it since is a new object (CR 400.7).
func TestDelayedSelfGraveyardSkipsReturnedSource(t *testing.T) {
	t.Parallel()
	e, src, n := delayedGraveMoves(t, true)
	if n != 0 {
		t.Fatalf("re-entered graveyard card moved %d times, want none (CR 603.7c)", n)
	}
	if got := e.G.Obj(src).Zone; got != state.ZGraveyard {
		t.Fatalf("source zone = %v, want graveyard", got)
	}
}

// TestDelayedSelfGraveyardControl: the card that stayed in the graveyard is
// still returned by the delayed trigger.
func TestDelayedSelfGraveyardControl(t *testing.T) {
	t.Parallel()
	e, src, n := delayedGraveMoves(t, false)
	if n != 1 {
		t.Fatalf("graveyard card moved %d times, want the one return", n)
	}
	if got := e.G.Obj(src).Zone; got != state.ZBattlefield {
		t.Fatalf("source zone = %v, want battlefield", got)
	}
}

// TestDelayedSelfRegistrationBlinkSkips: the source crossed the battlefield
// boundary after the delayed trigger registered but before it fired, so the
// body's Defined$ Self names the object that registered it (CR 603.7c), which
// is gone; the returned card is a new object (CR 400.7) and is not moved.
func TestDelayedSelfRegistrationBlinkSkips(t *testing.T) {
	t.Parallel()
	e, src := delayedSelfFixture(t)
	if len(e.G.Delayed) != 1 {
		t.Fatalf("precondition: delayed registrations = %d, want one", len(e.G.Delayed))
	}
	before := e.G.Obj(src).Incarnation
	e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZBattlefield, To: state.ZExile})
	e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZExile, To: state.ZBattlefield})
	o := e.G.Obj(src)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: source zone = %v, want battlefield after the round trip", o)
	}
	if o.Incarnation == before {
		t.Fatalf("precondition: incarnation = %d before and after the round trip, want a new one", before)
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
		t.Fatalf("source returned before the fire moved %d times in the delayed resolution, want none (CR 603.7c)", n)
	}
	if got := e.G.Obj(src); got == nil || got.Zone != state.ZBattlefield {
		t.Fatalf("returned source zone = %v after the delayed trigger, want battlefield", got)
	}
}
