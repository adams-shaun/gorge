package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestOptionalScryNoHostDeclinesWithoutLooking(t *testing.T) {
	h := &fakeHost{}
	h.g = state.NewGame(names(2))
	card := mkCard(t, "Name:DistinctTop\nTypes:Creature\nPT:2/2\nOracle:x\n")
	top := h.g.AddObject(card, 0)
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{top.ID})
	if got := h.g.Zone(state.ZLibrary, 0); len(got) != 1 || got[0] != top.ID {
		t.Fatalf("precondition: top card missing: %v", got)
	}
	effect := sa(t, "SP$ Scry | Defined$ You | ScryNum$ 1 | Optional$ True")
	Resolve(h, &Ctx{Controller: 0}, effect)
	if len(h.log) != 0 {
		t.Fatalf("no-host optional scry was not declined: events = %+v", h.log)
	}
	if got := h.g.Zone(state.ZLibrary, 0); len(got) != 1 || got[0] != top.ID {
		t.Fatalf("declined scry changed library: %v", got)
	}
}
