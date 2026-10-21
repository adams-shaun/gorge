package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// The engine records its internal status markers ("Shield", the regeneration
// shield, and "Deathtouched", the lethal-damage mark) as counters because it
// has no per-object status field, but they are not counters (CR 122.1). The
// every-kind readers must exclude them the same way hasCounters and
// proliferate do (cr-g81): a regenerating creature is not modified (CR 700.9)
// and "remove every counter" leaves the shield alone.

func TestModifiedIgnoresInternalMarkers(t *testing.T) {
	g, ids := board(t)
	bear := g.Obj(ids["myBear"])
	bear.AddCounter("Shield", 1)
	bear.AddCounter("Deathtouched", 1)
	if bear.Counter("Shield") != 1 || bear.Counter("Deathtouched") != 1 {
		t.Fatalf("fixture: markers not on the creature (Shield %d, Deathtouched %d)",
			bear.Counter("Shield"), bear.Counter("Deathtouched"))
	}
	if modifiedPermanent(g, bear, 0, 0) {
		t.Fatal("creature with only internal markers counts as modified (CR 700.9)")
	}

	bear.AddCounter("P1P1", 1)
	if !modifiedPermanent(g, bear, 0, 0) {
		t.Fatal("creature with a +1/+1 counter is not modified (CR 700.9)")
	}
}

func TestRemoveAllCountersKeepsShield(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	bear := g.Obj(ids["myBear"])
	bear.AddCounter("Shield", 1)
	bear.AddCounter("P1P1", 2)
	if bear.Counter("Shield") != 1 || bear.Counter("P1P1") != 2 {
		t.Fatalf("fixture: counters not on the creature (Shield %d, P1P1 %d)",
			bear.Counter("Shield"), bear.Counter("P1P1"))
	}

	Resolve(h, &Ctx{Controller: 0, Source: ids["myBear"]},
		sa(t, "DB$ RemoveCounter | Defined$ Self | CounterType$ All | CounterNum$ All"))

	if got := bear.Counter("P1P1"); got != 0 {
		t.Fatalf("P1P1 = %d, want 0 (CounterType$ All removes real counters)", got)
	}
	if got := bear.Counter("Shield"); got != 1 {
		t.Fatalf("Shield = %d, want 1 (regeneration shield is not a counter)", got)
	}
	for _, ev := range h.log {
		if ev.Kind == events.CounterChange && ev.Obj == bear.ID && ev.Counter == "Shield" {
			t.Fatalf("emitted a CounterChange for the internal Shield marker: %+v", ev)
		}
	}
}
