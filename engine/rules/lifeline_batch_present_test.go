package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Simultaneous departures are ONE event (CR 704.3): the intervening-if
// "if another creature is on the battlefield" of a non-departing Lifeline is
// read after all of them left, so with no creature remaining it triggers for
// none of them -- not once per member, each member being the other's "another
// creature" on the shared pre-batch board.
var lbLifeline = "Name:Lifeline Batch\nTypes:Artifact\n" +
	"T:Mode$ ChangesZone | ValidCard$ Card.Creature | Origin$ Battlefield | Destination$ Graveyard | IsPresent$ Creature | Execute$ TrigLBGain | TriggerZones$ Battlefield | TriggerDescription$ x\n" +
	"SVar:TrigLBGain:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"

const lbBear = "Name:Batch Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

func lbKillAll(t *testing.T, e *Engine, ids ...state.ObjID) {
	t.Helper()
	for _, id := range ids {
		e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 2})
	}
	e.checkStateBased()
	for _, id := range ids {
		if o := e.G.Obj(id); o.Zone != state.ZGraveyard {
			t.Fatalf("precondition: %d zone = %v, want graveyard", id, o.Zone)
		}
	}
}

func lbQueuedFor(e *Engine, src state.ObjID) int {
	n := 0
	for _, pt := range e.pendingTriggers {
		if pt.Source == src {
			n++
		}
	}
	return n
}

func TestLifelineBatchOfAllCreaturesQueuesNothing(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	lifeline := onBoard(t, e, 0, lbLifeline)
	a := onBoard(t, e, 0, lbBear)
	b := onBoard(t, e, 0, lbBear)
	lbKillAll(t, e, a, b)
	if n := lbQueuedFor(e, lifeline); n != 0 {
		t.Fatalf("Lifeline queued %d triggers for a batch that left no creature, want 0", n)
	}
}

func TestLifelineBatchWithASurvivorQueuesPerMember(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	lifeline := onBoard(t, e, 0, lbLifeline)
	a := onBoard(t, e, 0, lbBear)
	b := onBoard(t, e, 0, lbBear)
	survivor := onBoard(t, e, 1, lbBear)
	if o := e.G.Obj(survivor); o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("precondition: survivor on the battlefield")
	}
	lbKillAll(t, e, a, b)
	if n := lbQueuedFor(e, lifeline); n != 2 {
		t.Fatalf("Lifeline queued %d triggers with a creature remaining, want 2", n)
	}
}
