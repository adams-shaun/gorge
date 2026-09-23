package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

func TestOptionalScryAsksBeforeLookingAndDeclineSkips(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	card := mkCard(t, "Name:DistinctTop\nTypes:Creature\nPT:2/2\nOracle:x\n")
	top := h.g.AddObject(card, 0)
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{top.ID})
	if got := h.g.Zone(state.ZLibrary, 0); len(got) != 1 || got[0] != top.ID {
		t.Fatalf("precondition: top card missing: %v", got)
	}
	effect := sa(t, "SP$ Scry | Defined$ You | ScryNum$ 1 | Optional$ True")
	Resolve(h, &Ctx{Controller: 0}, effect)
	if h.asked == nil || h.asked.Kind != decision.KChoose || h.asked.ResumeKind != "scry_optional" {
		t.Fatalf("decision = %+v, want optional scry election", h.asked)
	}
	if len(h.log) != 0 {
		t.Fatalf("look occurred before choice: %+v", h.log)
	}
	Resolve(h, &Ctx{Controller: 0, ScryOpt: "no", Arrange: true, LibraryTarget: -1}, effect)
	if len(h.log) != 0 {
		t.Fatalf("declined scry emitted events: %+v", h.log)
	}
}
