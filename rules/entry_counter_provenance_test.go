package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestEntryCounterNoticePublishesAdder(t *testing.T) {
	walker := entryCounterWalker(t)
	e, cfg := tokenReplGame(t, 9401, walker)
	id := moveSeededCard(t, e, 0, walker, state.ZHand)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: walker not in hand: %+v", o)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Counter("LOYALTY") != 4 {
		t.Fatalf("precondition: walker entry = %+v, want battlefield with 4 loyalty", o)
	}
	found := false
	for _, row := range e.counterAddsThisTurn {
		if row.object.ID == id && row.kind == "LOYALTY" && row.amount == 4 && row.actor == 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("entry LOYALTY placement absent from ledger with seat 0 as adder: %+v", e.counterAddsThisTurn)
	}
	replayCheck(t, e, cfg)
}
