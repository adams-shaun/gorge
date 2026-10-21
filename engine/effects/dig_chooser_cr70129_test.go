package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDigOpponentTopRemainderAsksTheControllerAndNoHostKeepsOrder (cr-g08,
// CR 701.29a): a Dig over seat 1's library resolved by seat 0 with an
// omitted DestinationZone2$ and an explicit LibraryPosition2$ 0 leaves the
// window on TOP (the explicit position is not overwritten by the omitted
// zone's bottom default); seat 0 -- the controller -- gets the private look
// and is asked the top order (KArrange, Option.Kind "top", options owned by
// seat 1); and the R-9 no-host stand-in keeps the offered order, emitting no
// LibraryOrder.
func TestDigOpponentTopRemainderAsksTheControllerAndNoHostKeepsOrder(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	land := mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	ids := []state.ObjID{
		h.g.AddObject(land, 1).ID,
		h.g.AddObject(land, 1).ID,
		h.g.AddObject(land, 1).ID,
		h.g.AddObject(land, 1).ID,
	}
	h.g.SetZone(state.ZLibrary, 1, ids)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ Dig | Defined$ Opponent | DigNum$ 3 | ChangeNum$ 0 | DestinationZone$ Library | LibraryPosition2$ 0"))
	if lib := h.g.Zone(state.ZLibrary, 1); !slices.Equal(lib, ids) {
		t.Fatalf("seat 1 library = %v, want %v unchanged (LibraryPosition2$ 0 keeps the rest on top; no host keeps the offered order)", lib, ids)
	}
	for _, e := range h.log {
		if e.Kind == events.LibraryOrder {
			t.Fatalf("no-host stand-in emitted a LibraryOrder: %+v", e)
		}
		if e.Kind == events.Note && e.Secret && e.Text == "looks at the top of the library" && e.Player != 0 {
			t.Fatalf("private look went to player %d, want only the controller (0)", e.Player)
		}
	}
	d := h.asked
	if d == nil || d.Kind != decision.KArrange || d.ResumeKind != "dig_arrange" {
		t.Fatalf("asked = %+v, want the top-order KArrange", d)
	}
	if d.Player != 0 || d.Min != 3 || d.Max != 3 || len(d.Options) != 3 {
		t.Fatalf("top-order ask player/min/max/options = %d/%d/%d/%d, want 0/3/3/3", d.Player, d.Min, d.Max, len(d.Options))
	}
	for i, o := range d.Options {
		if o.Kind != "top" || o.Obj != ids[i] || o.Player != 1 {
			t.Fatalf("option %d = %+v, want Kind top, Obj %d, Player 1 (the library's owner)", i, o, ids[i])
		}
	}
}
