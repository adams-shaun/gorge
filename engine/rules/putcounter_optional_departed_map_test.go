package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPutCounterOptionalDepartedTriggeredMapPlacesNothing: Captain Marvel,
// Apex Avenger's "you may put the same number and kind of counters on her"
// (TriggeredCounterMap$, Defined$ Self, Optional$) resolving after she was
// exiled poses no election and puts no counter on the exiled card (CR 400.7).
// The election is skipped because no counter could land, so the unasked put
// must place nothing either.
func TestPutCounterOptionalDepartedTriggeredMapPlacesNothing(t *testing.T) {
	t.Parallel()
	e, cm, other := triggeredCounterMapGame(t, "Name:Non-Kree Test Creature\nTypes:Creature Human\nPT:2/2\nOracle:x\n")
	before := len(e.pendingTriggers)
	putTwoKinds(t, e, other)
	if n := len(e.pendingTriggers) - before; n != 1 {
		t.Fatalf("precondition: %d Captain Marvel triggers queued, want 1", n)
	}
	e.putTriggersOnStack()
	e.emit(events.Event{Kind: events.MoveZone, Obj: cm, From: state.ZBattlefield, To: state.ZExile})
	if o := e.G.Obj(cm); o == nil || o.Zone != state.ZExile {
		t.Fatalf("precondition: Captain Marvel should be in exile, got %+v", o)
	}
	mark := len(e.L.Events)
	e.resolveTop()
	resolved := false
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Resolve {
			resolved = true
		}
		if strings.Contains(ev.Text, "put_optional") {
			t.Fatalf("the may-put election was posed for an exiled source: %+v", ev)
		}
	}
	if !resolved {
		t.Fatal("precondition: the trigger did not resolve")
	}
	if d := e.Pending(); d != nil && d.ResumeKind == "put_optional" {
		t.Fatalf("pending = %+v: the may-put election is pending for an exiled source", d)
	}
	if evs := counterChangeEventsFor(e, cm); len(evs) != 0 {
		t.Fatalf("the exiled Captain Marvel took counters: %+v", evs)
	}
}
