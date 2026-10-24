package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTriggeredAttackerDefinedBatch pins that Defined$ TriggeredAttackerLKICopy
// on a Mode$ Attacks trigger binds only the attacker that matched the trigger,
// not every creature in the same DeclareAttackers event. Ordeal of Heliod's
// "put a +1/+1 counter on it" is the corpus spelling.
func TestTriggeredAttackerDefinedBatch(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	bear := onBoardReady(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	other := onBoardReady(t, e, 0, "Name:Beast\nManaCost:1 G\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")
	ordeal := onBoardCard(t, e, 0, corpusCard(t, "Ordeal of Heliod"))
	e.emit(events.Event{Kind: events.Attach, Obj: ordeal, IDs: []state.ObjID{bear}})
	if got := e.G.Obj(ordeal).AttachedTo; got != bear {
		t.Fatalf("precondition: Ordeal attached to %d, want the Bear %d", got, bear)
	}

	// Player is the DEFENDING seat; both creatures attack from seat 0.
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear, other}})
	for i := 0; i < 16; i++ {
		if len(e.pendingTriggers) == 0 && len(e.G.Stack) == 0 {
			break
		}
		e.putTriggersOnStack()
		if len(e.G.Stack) == 0 {
			break
		}
		e.resolveTop()
	}

	if got := e.G.Obj(bear).Counter("P1P1"); got != 1 {
		t.Errorf("enchanted attacker counters = %d, want 1", got)
	}
	if got := e.G.Obj(other).Counter("P1P1"); got != 0 {
		t.Errorf("other attacker counters = %d, want 0 (Defined$ TriggeredAttackerLKICopy must bind only the triggering attacker)", got)
	}
}
