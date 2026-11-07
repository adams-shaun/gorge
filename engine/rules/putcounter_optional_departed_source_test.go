package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPutCounterOptionalDepartedSourceNoAsk: an Optional$ PutCounter on
// Defined$ Self whose source left the battlefield after its trigger was
// pushed poses no "put a counter on it?" election (CR 400.7): the counter
// could not land on the card in the graveyard, so the ask would change
// nothing. Talus Paladin's entry trigger is pushed while it is on the
// battlefield; it then dies before the trigger resolves.
func TestPutCounterOptionalDepartedSourceNoAsk(t *testing.T) {
	t.Parallel()
	e, cfg, pal := gateFixture(t, 4711, "Talus Paladin")
	pal = gateMoveFromLibrary(t, e, "Talus Paladin", state.ZBattlefield)
	e.emit(events.Event{Kind: events.TriggerPush, Obj: pal, Player: 0, Amount: 0})
	e.emit(events.Event{Kind: events.MoveZone, Obj: pal, From: state.ZBattlefield, To: state.ZGraveyard})
	if o := e.G.Obj(pal); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Talus Paladin should be in the graveyard, got %+v", o)
	}
	mark := len(e.L.Events)
	e.resolveTop()
	resolved := false
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Resolve {
			resolved = true
		}
		// a posed election is either pending or, with no resolution run to
		// serve it, answered with its default under a note naming it
		if strings.Contains(ev.Text, "put_optional") {
			t.Fatalf("the may-put election was posed for a source that left the battlefield: %+v", ev)
		}
	}
	if !resolved {
		t.Fatalf("precondition: the trigger did not resolve")
	}
	if d := e.Pending(); d != nil && d.ResumeKind == "put_optional" {
		t.Fatalf("pending = %+v: the may-put election is pending for a departed source", d)
	}
	if evs := counterChangeEventsFor(e, pal); len(evs) != 0 {
		t.Fatalf("the departed Paladin took counters: %+v", evs)
	}
	replayCheck(t, e, cfg)
}
