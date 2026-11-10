package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestCountersReceivedThisTurnEngine runs La Forge's real X body against the
// engine's counter ledger: my placement marks the permanent, an opponent's
// placement on it does not, my placement on an opponent's permanent fails
// YouCtrl, and the TurnChange ends every mark.
func TestCountersReceivedThisTurnEngine(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	forge := onBoardCard(t, e, 0, yourCountersCard(t, "La Forge, Thoughtful as Ever"))
	mine := onBoardCard(t, e, 0, yourCountersCard(t, "Wakka, Devoted Guardian"))
	theirsMarkedByMe := onBoardCard(t, e, 1, yourCountersCard(t, "Wakka, Devoted Guardian"))
	markedByThem := onBoardCard(t, e, 0, yourCountersCard(t, "Wakka, Devoted Guardian"))

	body := svarBodyOf(t, e.G.Obj(forge).Face(), "X")
	if body != "Count$Valid Permanent.YouCtrl+countersReceivedThisTurn_GE1_Any_You" {
		t.Fatalf("test precondition: La Forge X = %q", body)
	}
	ctx := &effects.Ctx{Controller: 0, Source: forge}
	if n, ok := effects.EvalCountOK(e, ctx, body); !ok || n != 0 {
		t.Fatalf("baseline = %d (ok %v), want evaluated zero", n, ok)
	}

	prev := e.SetCounterAdder(0)
	e.emit(events.Event{Kind: events.CounterChange, Obj: mine, Counter: "P1P1", Amount: 1})
	e.emit(events.Event{Kind: events.CounterChange, Obj: mine, Counter: "LORE", Amount: 1})
	e.emit(events.Event{Kind: events.CounterChange, Obj: theirsMarkedByMe, Counter: "P1P1", Amount: 1})
	e.SetCounterAdder(1)
	e.emit(events.Event{Kind: events.CounterChange, Obj: markedByThem, Counter: "P1P1", Amount: 1})
	e.SetCounterAdder(prev)

	// Two kinds on one permanent are still one permanent; the opponent's mark
	// and my mark on their permanent do not count.
	if n, ok := effects.EvalCountOK(e, ctx, body); !ok || n != 1 {
		t.Fatalf("after placements = %d (ok %v), want 1", n, ok)
	}
	clone := e.Clone()
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	if n, _ := effects.EvalCountOK(e, ctx, body); n != 0 {
		t.Fatalf("after the turn change = %d, want 0", n)
	}
	if n, _ := effects.EvalCountOK(clone, ctx, body); n != 1 {
		t.Fatalf("clone changed when the original reset: %d, want 1", n)
	}
}
