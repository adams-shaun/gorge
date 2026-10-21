package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// cr-g64: a Dig take the chooser LOOKED at (a real choice records the look)
// and then exiles face down records the chooser in the may-look layout, so
// the view shows that seat the face (CR 406.3). A blind take (no look) keeps
// the plain marker.
func TestDigLookedFaceDownTakeRecordsTheChooser(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	land := mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	ids := []state.ObjID{h.g.AddObject(land, 0).ID, h.g.AddObject(land, 0).ID}
	h.g.SetZone(state.ZLibrary, 0, ids)
	Resolve(h, &Ctx{Controller: 0},
		sa(t, "SP$ Dig | Defined$ You | DigNum$ 2 | ChangeNum$ 1 | DestinationZone$ Exile | ExileFaceDown$ True"))
	moves := digSecretMoves(t, h.log)
	if len(moves) != 1 || moves[0].To != state.ZExile {
		t.Fatalf("looked-at Dig exile moves = %+v, want one exile move", moves)
	}
	ev := moves[0]
	if ev.Counter != "exiled_with_face_down_maylook" {
		t.Fatalf("looked-at Dig exile Counter = %q, want the may-look marker", ev.Counter)
	}
	if ev.Amount != 0 {
		t.Fatalf("looked-at Dig exile Amount = %d, want the chooser 0 (the looker)", ev.Amount)
	}
	if o := h.g.Obj(ev.Obj); o == nil || !o.FaceDown || !o.HasMayLook || o.MayLookPlayer != 0 {
		t.Fatalf("looked-at Dig exile state = %+v, want FaceDown with seat 0 the looker", o)
	}
	if ev.Kind != events.MoveZone {
		t.Fatalf("looked-at Dig exile event kind = %v, want MoveZone", ev.Kind)
	}
}
