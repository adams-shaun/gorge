package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A delayed end-step registration over an EXILE capture whose Execute counts
// Count$ValidExile Card.IsTriggerRemembered (Dragonhawk, Fate's Tempest's "at
// the beginning of your next end step, NICKNAME deals 2 damage to each
// opponent for each of those cards that are still exiled"): CR 400.7, a
// remembered card that left exile and came back is a NEW object, so it is no
// longer one of "those cards" and must not count. The registration stamps
// each remembered object's Incarnation and ZoneGen (state.DelayedTrigger.
// RememberedStamps, events' DelayedRegister fold); the read must consult
// them, not the surviving ObjID.

// stampExileSource's Execute is Dragonhawk's own damage body: the count is
// doubled, so one remembered card deals 2 damage and two deal 4.
const stampExileSource = "Name:Dig promise\nTypes:Creature\nPT:4/4\n" +
	"SVar:TrigDamage:DB$ DamageAll | ValidPlayers$ Player.Opponent | NumDmg$ Y\n" +
	"SVar:Y:Count$ValidExile Card.IsTriggerRemembered/Twice\nOracle:x\n"

func stampExileDelayedSA() *cards.SA {
	return &cards.SA{Kind: "DB", API: "DelayedTrigger", Params: map[string]string{
		"Mode":    "Phase",
		"Phase":   "End of Turn",
		"Execute": "TrigDamage",
	}}
}

// registerStampExileDelayed resolves the DB$ DelayedTrigger with captured as
// the resolving chain's Remembered, the way Dragonhawk's dig chain supplies
// the cards it exiled.
func registerStampExileDelayed(t *testing.T, e *Engine, src state.ObjID, captured ...state.ObjID) {
	t.Helper()
	rem := make([]state.Target, 0, len(captured))
	for _, id := range captured {
		rem = append(rem, state.Target{Obj: id})
	}
	effects.Resolve(e, &effects.Ctx{Source: src, Controller: 0, Remembered: rem}, stampExileDelayedSA())
}

func TestDelayedRememberedStampCountSkipsReturnedExileCard(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	src := onBoard(t, e, 0, stampExileSource)
	returned := inZoneCard(t, e, 0, state.ZExile, "Name:Returned card\nTypes:Creature\nPT:2/2\nOracle:x\n")
	stable := inZoneCard(t, e, 0, state.ZExile, "Name:Stable card\nTypes:Creature\nPT:2/2\nOracle:x\n")
	if e.G.Obj(src).Zone != state.ZBattlefield || e.G.Obj(returned).Zone != state.ZExile ||
		e.G.Obj(stable).Zone != state.ZExile || e.G.Obj(returned).ID == e.G.Obj(stable).ID {
		t.Fatal("precondition: source on battlefield, two distinct cards in exile required")
	}

	registerStampExileDelayed(t, e, src, returned, stable)

	if len(e.G.Delayed) != 1 {
		t.Fatalf("precondition: registrations = %d, want 1", len(e.G.Delayed))
	}
	dt := e.G.Delayed[0]
	if len(dt.Remembered) != 2 || len(dt.RememberedStamps) != 2 {
		t.Fatalf("precondition: capture=%+v stamps=%+v, want both cards remembered AND stamped",
			dt.Remembered, dt.RememberedStamps)
	}

	// The remembered card leaves exile and comes back: same ObjID, a new
	// object (CR 400.7) -- its ZoneGen no longer matches the registration's.
	e.emit(eventMove(returned, state.ZExile, state.ZBattlefield))
	e.emit(eventMove(returned, state.ZBattlefield, state.ZExile))
	if e.G.Obj(returned).Zone != state.ZExile {
		t.Fatalf("precondition: returned card is in %v, want exile", e.G.Obj(returned).Zone)
	}
	if !movedStamp(e, &dt, returned) {
		t.Fatalf("precondition: returned card's stamps still match registration %+v; it must have moved", dt.RememberedStamps)
	}

	life := e.G.Players[1].Life
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepEnd})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pending=%d, want the end-step execute", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	if len(e.G.Stack) == 0 {
		t.Fatal("no delayed execute on stack")
	}
	e.resolveTop()
	if got := life - e.G.Players[1].Life; got != 2 {
		t.Errorf("damage %d, want 2: the returned card is a new object (CR 400.7), only the stable card counts", got)
	}
}

// The control: both remembered cards stayed in exile, both count.
func TestDelayedRememberedStampCountKeepsStableExileCards(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	src := onBoard(t, e, 0, stampExileSource)
	one := inZoneCard(t, e, 0, state.ZExile, "Name:Stable one\nTypes:Creature\nPT:2/2\nOracle:x\n")
	two := inZoneCard(t, e, 0, state.ZExile, "Name:Stable two\nTypes:Creature\nPT:2/2\nOracle:x\n")

	registerStampExileDelayed(t, e, src, one, two)

	if len(e.G.Delayed) != 1 {
		t.Fatalf("precondition: registrations = %d, want 1", len(e.G.Delayed))
	}
	dt := e.G.Delayed[0]
	if len(dt.RememberedStamps) != 2 {
		t.Fatalf("precondition: stamps=%+v, want both cards stamped", dt.RememberedStamps)
	}
	if movedStamp(e, &dt, one) || movedStamp(e, &dt, two) {
		t.Fatalf("precondition: stamps moved (%+v) with no card moved", dt.RememberedStamps)
	}

	life := e.G.Players[1].Life
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepEnd})
	e.putTriggersOnStack()
	if len(e.G.Stack) == 0 {
		t.Fatal("no delayed execute on stack")
	}
	e.resolveTop()
	if got := life - e.G.Players[1].Life; got != 4 {
		t.Errorf("damage %d, want 4: both stable remembered cards still count", got)
	}
}

// movedStamp reports whether the object has left the stamp the registration
// recorded for it (CR 400.7).
func movedStamp(e *Engine, dt *state.DelayedTrigger, id state.ObjID) bool {
	o := e.G.Obj(id)
	for _, s := range dt.RememberedStamps {
		if s.Obj == id {
			return s.Incarnation != o.Incarnation || s.ZoneGen != o.ZoneGen
		}
	}
	return true
}
