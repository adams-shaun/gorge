package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestSearchToBattlefieldEntersWithCounters is Captain Jean-Luc Picard's
// library search: a ChangeZone with Origin$ Library, Destination$ Battlefield,
// WithCountersType$ CHARGE and WithCountersAmount$ 7 must put the found
// Spacecraft onto the battlefield with the seven counters as part of the entry
// (CR 122.6), not as a separate counter event after it. The MoveZone event
// must carry the entry-counter payload itself and no CounterChange may follow
// the move; rules/search_station_entry_test.go asserts the Station 7+
// characteristic change those entry counters buy.
func TestSearchToBattlefieldEntersWithCounters(t *testing.T) {
	reg := hostlessSearchTestRegistry(t)
	picard, ok := reg.Lookup("Captain Jean-Luc Picard")
	if !ok {
		t.Fatal("missing corpus Captain Jean-Luc Picard")
	}
	ship, ok := reg.Lookup("Sledge-Class Seedship")
	if !ok {
		t.Fatal("missing corpus Sledge-Class Seedship")
	}
	bear, ok := reg.Lookup("Grizzly Bears")
	if !ok {
		t.Fatal("missing corpus Grizzly Bears")
	}

	h := &endureTapeHost{fakeHost: newHost(t, 2), answer: 0}
	src := h.g.AddObject(picard, 0)
	nonmatch := h.g.AddObject(bear, 0)
	target := h.g.AddObject(ship, 0)
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{nonmatch.ID, target.ID})
	h.g.Obj(nonmatch.ID).Zone = state.ZLibrary
	h.g.Obj(target.ID).Zone = state.ZLibrary
	if got := h.g.Obj(target.ID).Counter("CHARGE"); got != 0 {
		t.Fatalf("precondition: searched Spacecraft starts with %d CHARGE counters, want 0", got)
	}

	Resolve(h, &Ctx{Source: src.ID, Controller: 0, SVars: picard.Faces[0].SVars}, picard.Faces[0].Abilities[0])

	if got := h.g.Obj(target.ID).Zone; got != state.ZBattlefield {
		t.Fatalf("searched Spacecraft zone = %s, want battlefield", got)
	}
	var move *events.Event
	for i := range h.log {
		ev := &h.log[i]
		if ev.Kind == events.MoveZone && ev.Obj == target.ID && ev.From == state.ZLibrary && ev.To == state.ZBattlefield {
			move = ev
		}
	}
	if move == nil {
		t.Fatal("no library-to-battlefield MoveZone event for the searched Spacecraft")
	}
	want := events.EntryCounterPairs(events.EntryCounterGrant{Kind: "CHARGE", Amount: 7})
	if !slices.Equal(move.Pairs, want) {
		t.Fatalf("MoveZone entry-counter pairs = %v, want %v (the counters are part of the entry, CR 122.6)", move.Pairs, want)
	}
	for _, ev := range h.log {
		if ev.Kind == events.CounterChange && ev.Obj == target.ID {
			t.Fatalf("CHARGE counters placed after the move as a separate CounterChange: %+v", ev)
		}
	}
	if got := h.g.Obj(target.ID).Counter("CHARGE"); got != 7 {
		t.Fatalf("searched Spacecraft CHARGE counters = %d, want 7", got)
	}
	if got := h.g.Obj(nonmatch.ID).Zone; got != state.ZLibrary {
		t.Fatalf("non-Spacecraft zone = %s, want library (the ChangeType$ filter)", got)
	}
}
