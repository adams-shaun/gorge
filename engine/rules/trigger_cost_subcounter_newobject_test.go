package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 400.7: a triggered ability's "remove a counter from it" cost names the
// permanent that triggered. Once that permanent has left and come back it is a
// new object, so the counters now on the same ObjID are not the ability's to
// remove and the window must not offer to pay (Kappa Tech-Wrecker bounced and
// replayed by Ninjutsu while its trigger waits on the stack).
func TestTriggeredCostSubCounterSourceBouncedIsUnpayable(t *testing.T) {
	e, walker := subCounterFixture(t, "Slumbering Walker", "M1M1", 2)
	searchMoveByName(t, e, "Grizzly Bears", state.ZGraveyard)
	subCounterCostAsk(t, e)
	tc := e.triggerCost
	if tc == nil {
		t.Fatal("no trigger-cost window is open (fixture)")
	}
	if !tc.triggerSubCounterPayable(e, tc.amount) {
		t.Fatal("precondition: the untouched source holding the counters must be payable")
	}
	inc := e.G.Obj(walker).Incarnation
	e.emit(events.Event{Kind: events.MoveZone, Obj: walker, From: state.ZBattlefield, To: state.ZHand})
	e.emit(events.Event{Kind: events.MoveZone, Obj: walker, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.CounterChange, Obj: walker, Counter: "M1M1", Amount: 2 - e.G.Obj(walker).Counter("M1M1")})
	o := e.G.Obj(walker)
	if o.Zone != state.ZBattlefield || o.Counter("M1M1") != 2 || o.Incarnation == inc {
		t.Fatalf("precondition: the walker must be a new incarnation with 2 counters on the battlefield: zone %v counters %d incarnation %d->%d", o.Zone, o.Counter("M1M1"), inc, o.Incarnation)
	}
	if tc.triggerSubCounterPayable(e, tc.amount) {
		t.Fatal("a bounced source's new incarnation paid the old trigger's counter cost")
	}
}
