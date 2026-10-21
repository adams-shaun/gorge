package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestHideawayNoHostExileIsTheControllersSecret (cr-g47): the R-9 no-host
// Hideaway stand-in, resolved by seat 1 over its own library, exiles the
// first card face down with a Secret MoveZone whose Player is seat 1 -- the
// controller who looked (CR 702.75a). An omitted Player reads as seat 0, and
// redaction rule 1 would hand seat 0 the hidden card.
func TestHideawayNoHostExileIsTheControllersSecret(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	land := mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	var ids []state.ObjID
	for i := 0; i < 5; i++ {
		ids = append(ids, h.g.AddObject(land, 1).ID)
	}
	h.g.SetZone(state.ZLibrary, 1, ids)
	Resolve(h, &Ctx{Controller: 1}, sa(t, "SP$ Hideaway | Amount$ 4"))
	n := 0
	for _, e := range h.log {
		if e.Kind != events.MoveZone || e.To != state.ZExile {
			continue
		}
		n++
		if !e.Secret || e.Player != 1 || e.Obj != ids[0] {
			t.Fatalf("Hideaway exile = %+v, want Secret, Player 1 (the controller), Obj %d", e, ids[0])
		}
	}
	if n != 1 {
		t.Fatalf("Hideaway exiled %d cards, want 1", n)
	}
}
