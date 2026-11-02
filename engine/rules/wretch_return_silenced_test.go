package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A Retched Wretch that came back from its own dies trigger has lost every
// ability for good (CR 613.1f), so killing it a second time, with -1/-1
// counters on it again, triggers nothing (CR 603.10a looks back at the
// object as it last stood, and it stood with no abilities).
func TestReturnedWretchDiesWithoutItsTrigger(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Retched Wretch")}, nil)
	w := moveCorpusCard(t, e, "Retched Wretch", 0, state.ZBattlefield)
	e.emit(events.Event{Kind: events.CounterChange, Obj: w, Counter: "M1M1", Amount: 2})
	e.checkStateBased()
	e.priorityRound()
	if len(e.G.Stack) != 1 {
		t.Fatalf("first death: %d stack objects, want its dies trigger", len(e.G.Stack))
	}
	passUntilStackEmpty(t, e, 40)
	o := e.G.Obj(w)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("the Wretch did not return: %+v", o)
	}
	if !e.printedAbilitiesLost(o) {
		t.Fatal("the returned Wretch still has its printed abilities")
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: w, Counter: "M1M1", Amount: 4})
	e.checkStateBased()
	e.priorityRound()
	if n := len(e.G.Stack); n != 0 {
		t.Errorf("second death: %d stack objects, want none", n)
	}
}
