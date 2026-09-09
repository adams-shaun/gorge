package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestLibrarySearchNoHostFindsNothingAndShuffles(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	wilds, ok := reg.Lookup("Evolving Wilds")
	if !ok {
		t.Fatal("missing corpus Evolving Wilds")
	}
	forest, ok := reg.Lookup("Forest")
	if !ok {
		t.Fatal("missing corpus Forest")
	}
	h := newHost(t, 2)
	src := h.g.AddObject(wilds, 0)
	basic := h.g.AddObject(forest, 0)
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{src.ID})
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{basic.ID})
	h.g.Obj(src.ID).Zone = state.ZBattlefield
	h.g.Obj(basic.ID).Zone = state.ZLibrary

	var searchSA = wilds.Faces[0].Abilities[0]
	Resolve(h, &Ctx{Source: src.ID, Controller: 0}, searchSA)

	if h.g.Obj(basic.ID).Zone != state.ZLibrary {
		t.Fatalf("no-host stand-in moved the basic to %s, want library", h.g.Obj(basic.ID).Zone)
	}
	moves, shuffles := 0, 0
	for _, ev := range h.log {
		if ev.Kind == events.MoveZone {
			moves++
		}
		if ev.Kind == events.Shuffle && ev.Player == 0 {
			shuffles++
		}
	}
	if moves != 0 || shuffles != 1 {
		t.Fatalf("no-host search emitted %d moves and %d shuffles, want 0/1: %+v", moves, shuffles, h.log)
	}
	if h.Suspended() {
		t.Fatal("no-host search suspended")
	}
}
