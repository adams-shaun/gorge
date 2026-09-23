package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestScryInstructionEmitsTriggerMarker(t *testing.T) {
	e, _, id := scryFixture(t, 8201)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: Scry source %v must be in hand: %+v", id, o)
	}
	before := len(e.L.Events)
	scryDecision(t, e, id)
	found := false
	for _, ev := range e.L.Events[before:] {
		if ev.Kind == events.Scry {
			found = true
			if ev.Player != 0 || ev.Obj != id {
				t.Fatalf("Scry marker = %+v, want player 0/source %v", ev, id)
			}
		}
	}
	if !found {
		t.Fatal("Scry resolution emitted no events.Scry marker")
	}
}
