package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestStackCopyKeepsTheGiftPromise: the gift promise is an additional cost
// (CR 702.174a) and a copy copies the additional costs paid for the spell
// (CR 707.10), so a copy of a promised-gift spell reads PromisedGift and
// gives the same opponent the gift, while a copy of a spell whose gift was
// declined has no promise.
func TestStackCopyKeepsTheGiftPromise(t *testing.T) {
	for _, promise := range []bool{true, false} {
		g, id := gameWithOneCard(t)
		Apply(g, Event{Kind: PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack})
		ev := Event{Kind: GiftPromise, Obj: id, Player: 1}
		if promise {
			ev.Amount = 1
		}
		Apply(g, ev)
		Apply(g, Event{Kind: StackCopy, Obj: id, Player: 0})
		if len(g.Stack) != 2 {
			t.Fatalf("stack %v, want the original and its copy", g.Stack)
		}
		c := g.Obj(g.Stack[1])
		if c == nil || !c.IsCopy {
			t.Fatalf("top of stack is not a copy: %+v", c)
		}
		if got := c.CastFlags&state.FlagPromisedGift != 0; got != promise {
			t.Errorf("promise=%v: copy PromisedGift = %v", promise, got)
		}
		if promise && c.GiftPromisedTo != 1 {
			t.Errorf("copy's gift receiver = seat %d, want 1", c.GiftPromisedTo)
		}
	}
}
