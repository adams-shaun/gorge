package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// maxDistinctTrigSrc is a permanent or spell with one triggered ability, so a
// TriggerPush of it mints an ability object whose Source is the card.
const maxDistinctTrigSrc = "Name:Trigger Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDraw | TriggerDescription$ x\n" +
	"SVar:TrigDraw:DB$ Draw | NumCards$ 1\nOracle:x\n"

// TestMaxDistinctOnStackHeadTracksTheHighWaterMark: Geometric Weird's
// Count$MaxDistinctOnStack is the greatest number of distinct SOURCES with an
// object on the stack at once this turn (CR 405). A spell and its own ability
// count once, two abilities of one permanent count once, the mark survives the
// stack emptying, and it resets at the turn boundary.
func TestMaxDistinctOnStackHeadTracksTheHighWaterMark(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	src := onBoard(t, e, 0, historyBearSrc)
	head := func(what string, want int32) {
		t.Helper()
		if got := historyEval(t, e, src, nil, "Count$MaxDistinctOnStack"); got != want {
			t.Fatalf("%s: Count$MaxDistinctOnStack = %d, want %d", what, got, want)
		}
	}
	push := func(p state.PlayerID, script string) state.ObjID {
		t.Helper()
		o := e.G.AddObject(card(t, script), p)
		e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZStack})
		return o.ID
	}
	head("empty stack", 0)
	spell := push(0, maxDistinctTrigSrc)
	head("one spell", 1)
	e.emit(events.Event{Kind: events.TriggerPush, Obj: spell, Player: 0, Amount: 0})
	if len(e.G.Stack) != 2 {
		t.Fatalf("precondition: stack holds %d objects, want the spell and its ability", len(e.G.Stack))
	}
	head("a spell and its own ability share one source", 1)
	perm := onBoard(t, e, 1, maxDistinctTrigSrc)
	e.emit(events.Event{Kind: events.TriggerPush, Obj: perm, Player: 1, Amount: 0})
	head("a second source", 2)
	e.emit(events.Event{Kind: events.TriggerPush, Obj: perm, Player: 1, Amount: 0})
	if len(e.G.Stack) != 4 {
		t.Fatalf("precondition: stack holds %d objects, want 4", len(e.G.Stack))
	}
	head("two abilities of one permanent count once", 2)
	push(1, historyBearSrc)
	head("a third source", 3)
	clearStack := func() {
		t.Helper()
		for len(e.G.Stack) > 0 {
			e.emit(events.Event{Kind: events.MoveZone, Obj: e.G.Stack[len(e.G.Stack)-1], From: state.ZStack, To: state.ZExile})
		}
	}
	clearStack()
	head("the mark survives the stack emptying", 3)
	push(0, historyBearSrc)
	head("a lower later stack leaves the mark", 3)
	clearStack()
	e.emit(events.Event{Kind: events.TurnChange, Player: 1})
	head("a new turn", 0)
}
