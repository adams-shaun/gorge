package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestSearchToBattlefieldEntersWithCounters is Captain Jean-Luc Picard's
// library search: a ChangeZone with Origin$ Library, Destination$ Battlefield,
// WithCountersType$ CHARGE and WithCountersAmount$ 7 must put the found
// Spacecraft onto the battlefield with the seven counters as part of the entry
// (CR 122.6), not as a separate counter event after it.
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

	Resolve(h, &Ctx{Source: src.ID, Controller: 0, SVars: picard.Faces[0].SVars}, picard.Faces[0].Abilities[0])

	if got := h.g.Obj(target.ID).Zone; got != state.ZBattlefield {
		t.Fatalf("searched Spacecraft zone = %s, want battlefield", got)
	}
	if got := h.g.Obj(target.ID).Counter("CHARGE"); got != 7 {
		t.Fatalf("searched Spacecraft CHARGE counters = %d, want 7", got)
	}
	if got := h.g.Obj(nonmatch.ID).Zone; got != state.ZLibrary {
		t.Fatalf("non-Spacecraft zone = %s, want library (the ChangeType$ filter)", got)
	}
}
