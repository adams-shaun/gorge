package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Hunter's Talent has no opposing creature for its chained target, so its trigger never goes
// on the stack (CR 603.3d) and Emberheart Challenger, answered as its root target, did not
// become a target: no Valiant trigger, and the next targeting this turn is the first.
func TestTriggerWithoutAChainTargetTargetsNothing(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusCard(t, "Hunter's Talent"), corpusCard(t, "Might of the Meek"))
	challenger := addToBattlefield(t, e, corpusCard(t, "Emberheart Challenger"), 0)
	talent, might := e.G.Zone(state.ZHand, 0)[0], e.G.Zone(state.ZHand, 0)[1]
	e.G.Players[0].Pool[state.MG], e.G.Players[0].Pool[state.MC] = 1, 1
	castMode(t, e, talent, "")
	passUntilTarget(t, e, 20)
	pickTarget(t, e, challenger)
	if n := len(e.pendingTriggers); n != 0 {
		t.Fatalf("%d triggers queued by a target of an ability that never went on the stack", n)
	}
	if n := len(e.G.Stack); n != 0 {
		t.Fatalf("%d objects on the stack, want the trigger gone", n)
	}
	e.G.Players[0].Pool[state.MR] = 1
	castMode(t, e, might, "")
	passUntilTarget(t, e, 1)
	pickTarget(t, e, challenger)
	if n := len(e.pendingTriggers); n != 2 {
		t.Fatalf("%d triggers queued by the Might of the Meek cast, want its prowess and Valiant", n)
	}
}
