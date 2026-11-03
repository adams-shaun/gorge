package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestAtEOTDelayedTriggerOverGoneTokenStillTriggers pins CR 603.7 and
// CR 603.7c: Old Hob's "destroy it at the beginning of the next end step" is a
// delayed triggered ability that triggers when the end step begins. When the
// token it names has already left the game the ability still triggers and
// goes on the stack (players get priority with it there); on resolution it
// does nothing, because its object is gone.
func TestAtEOTDelayedTriggerOverGoneTokenStillTriggers(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := ateotEngine(t, reg, "Old Hob, Alleycat Blues")
	hob := ateotFind(t, e, "Old Hob, Alleycat Blues", 0)
	ateotTo(t, e, hob, state.ZLibrary, state.ZBattlefield)

	ateotDriveToStep(t, e, e.G.Turn, e.G.Active, state.StepBeginCombat)
	for e.putTriggersOnStack() {
		answerTriggerOrders(t, e)
		passUntilStackEmpty(t, e, 20)
	}
	passUntilStackEmpty(t, e, 20)

	var token state.ObjID
	for i := range e.G.Objs {
		if o := &e.G.Objs[i]; o.IsToken && o.Zone == state.ZBattlefield {
			token = o.ID
		}
	}
	if token == 0 || len(e.G.Delayed) != 1 {
		t.Fatalf("token %d, delayed %d; want a token and one registration", token, len(e.G.Delayed))
	}

	// The token dies before the end step.
	ateotTo(t, e, token, state.ZBattlefield, state.ZGraveyard)
	replayCheck(t, e, cfg)

	ateotDriveToStep(t, e, e.G.Turn, e.G.Active, state.StepEnd)
	for e.putTriggersOnStack() {
		answerTriggerOrders(t, e)
		if len(e.G.Stack) != 0 {
			break
		}
	}
	if len(e.G.Stack) == 0 {
		t.Fatal("the delayed destroy never reached the stack over the gone token")
	}
	top := e.G.Obj(e.G.Stack[len(e.G.Stack)-1])
	if top == nil || top.Ability == nil || top.Source != hob {
		t.Fatalf("stack top %+v, want Old Hob's delayed ability", top)
	}
	replayCheck(t, e, cfg)
	passUntilStackEmpty(t, e, 20)
	if z := e.G.Obj(hob).Zone; z != state.ZBattlefield {
		t.Fatalf("Old Hob zone %s after the empty trigger resolved", z)
	}
	replayCheck(t, e, cfg)
}
