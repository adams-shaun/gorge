package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A trigger whose own resolution exiles the object that fired it names the exiled
// object in its delayed return (CR 400.7j, 603.7c): a Barkform Harvester, a Bird by
// changeling, entering under Salvation Swan exiles itself and comes back at the end
// step. TestDelayedReturnSkipsTheObjectReturnedWhileItsTriggerWaited is the other
// side: a move before the resolution started leaves nothing for the return to find.
func TestDelayedReturnFindsTheObjectItsOwnTriggerExiled(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Salvation Swan"), corpusAlternativeCard(t, "Barkform Harvester"))
	hand := e.G.Zone(state.ZHand, 0)
	swan, bird := hand[0], hand[1]
	placeOnBattlefield(t, e, swan)
	e.emit(events.Event{Kind: events.MoveZone, Obj: bird, From: state.ZHand, To: state.ZBattlefield})
	if n := len(e.pendingTriggers); n != 1 {
		t.Fatalf("the Bird entering queued %d triggers, want the Swan's 1", n)
	}
	e.putTriggersOnStack()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("the Swan's trigger asked %v, want its target", d)
	}
	answerAsk(t, e, optionIdxForObj(t, d, bird))
	entered := e.G.Obj(bird).Incarnation
	drainQueuedTriggers(t, e)
	o := e.G.Obj(bird)
	if o.Zone != state.ZExile || o.Incarnation == entered {
		t.Fatalf("the trigger left the Bird in zone %v at incarnation %d", o.Zone, o.Incarnation)
	}
	passUntil(t, e, func() bool { return len(e.G.Delayed) == 0 && len(e.G.Stack) == 0 })
	o = e.G.Obj(bird)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("the delayed return left the Bird in zone %v, want the battlefield", o.Zone)
	}
	if got := o.Counter("Flying"); got != 1 {
		t.Fatalf("the Bird returned with %d flying counters, want 1", got)
	}
}
