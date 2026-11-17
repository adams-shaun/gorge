package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// drainChampion answers every non-priority decision with choice 0 until the
// stack is empty, returning how many decisions it answered.
func drainChampion(t *testing.T, e *Engine) int {
	t.Helper()
	asked := 0
	for range 8 {
		d := passUntilNonPriority(t, e, 60)
		if d == nil {
			break
		}
		asked++
		submitChoices(t, e, 0)
		if len(e.G.Stack) == 0 {
			break
		}
	}
	passUntilStackEmpty(t, e, 60)
	return asked
}

// CR 702.72a: Champion a Goblin. The Mob's enters trigger asks for another
// Goblin to exile (else the Mob is sacrificed); when the Mob leaves the
// battlefield the exiled card returns under its owner's control.
func TestChampionExilesAndReturns(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)

	t.Run("exiles a Goblin and returns it when the Mob dies", func(t *testing.T) {
		e, cfg := proliferateEngine(t, reg, "Boggart Mob", "Goblin Piker")
		piker := putNamedOnBattlefield(t, e, "Goblin Piker")
		mob := putNamedOnBattlefield(t, e, "Boggart Mob")
		if o := e.G.Obj(piker); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: Goblin Piker = %+v, want battlefield", o)
		}
		if asked := drainChampion(t, e); asked == 0 {
			t.Fatalf("the Champion trigger asked nothing")
		}
		if o := e.G.Obj(mob); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("Boggart Mob = %+v, want it kept on the battlefield", o)
		}
		if o := e.G.Obj(piker); o == nil || o.Zone != state.ZExile {
			t.Fatalf("Goblin Piker = %+v, want exile (championed)", o)
		}
		e.emit(events.Event{Kind: events.Damage, Obj: mob, Amount: 99})
		e.checkStateBased()
		e.priorityRound()
		if o := e.G.Obj(mob); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("Boggart Mob = %+v, want graveyard", o)
		}
		passUntilStackEmpty(t, e, 60)
		o := e.G.Obj(piker)
		if o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
			t.Fatalf("Goblin Piker = %+v, want back on seat 0's battlefield", o)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("sacrificed with no other Goblin", func(t *testing.T) {
		e, cfg := proliferateEngine(t, reg, "Boggart Mob")
		mob := putNamedOnBattlefield(t, e, "Boggart Mob")
		drainChampion(t, e)
		passUntilStackEmpty(t, e, 60)
		if o := e.G.Obj(mob); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("Boggart Mob = %+v, want sacrificed to the graveyard", o)
		}
		replayCheck(t, e, cfg)
	})
}
