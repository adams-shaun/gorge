package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestBlinkedSiegeIsNotDefeatedByItsOldTrigger: CR 400.7. The Siege that took
// its last defense counter is blinked while its defeat trigger is on the
// stack. It returns as a new permanent with its printed defense; the trigger
// belongs to the old one, so on resolution it must not exile the healthy new
// Siege nor offer the free cast.
func TestBlinkedSiegeIsNotDefeatedByItsOldTrigger(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, id := battleBoard(t, reg, "Invasion of Tolvada")
	printed := e.G.Obj(id).Counter("DEFENSE")
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "DEFENSE", Amount: -printed})
	e.putTriggersOnStack()
	if !siegeDefeatPending(e.G, e.pendingTriggers, id) {
		t.Fatalf("precondition: no Siege defeat ability on the stack (%v)", e.G.Stack)
	}
	var trig *state.Object
	for _, sid := range e.G.Stack {
		if so := e.G.Obj(sid); so != nil && so.Ability != nil && so.Source == id {
			trig = so
		}
	}
	if trig == nil || trig.SourceIncarnation == 0 {
		t.Fatalf("precondition: the defeat ability carries no source-incarnation stamp (%+v)", trig)
	}
	before := e.G.Obj(id).Incarnation
	// Blink: exile, then back to the battlefield under the same object id.
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZExile})
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZExile, To: state.ZBattlefield})
	o := e.G.Obj(id)
	if o.Incarnation == before {
		t.Fatalf("precondition: the blink did not make a new permanent (incarnation %d)", before)
	}
	if o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: blinked Siege is in %v, want the battlefield", o.Zone)
	}
	if o.Counter("DEFENSE") == 0 {
		e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "DEFENSE", Amount: printed})
	}
	if got := e.G.Obj(id).Counter("DEFENSE"); got != printed {
		t.Fatalf("precondition: new Siege has %d defense, want %d", got, printed)
	}
	for i := 0; i < 8 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			break
		}
		if d.Kind == decision.KChoose {
			t.Fatalf("a cast ask was posed for the old Siege's trigger: %+v", d)
		}
		if d.Kind != decision.KPriority {
			break
		}
		passPriority(t, e)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("precondition: the Siege defeat ability never resolved, stack %v pending %+v", e.G.Stack, e.Pending())
	}
	if got := e.G.Obj(id).Zone; got != state.ZBattlefield {
		t.Fatalf("the healthy new Siege is in %v, want the battlefield", got)
	}
}
