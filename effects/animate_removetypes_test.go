package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestAnimateRemoveTypesRegistersTypeRemoval(t *testing.T) {
	h, c := animateRemoveHost(t)
	card := mkCard(t, "Name:Weeping Angel\nManaCost:1 U B\nTypes:Artifact Creature Alien Angel\nPT:2/2\nOracle:x\n")
	o := h.g.AddObject(card, 0)
	o.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), o.ID))
	c.Source = o.ID
	if o.Zone != state.ZBattlefield || len(card.Faces) == 0 || len(card.Faces[0].Types) < 2 {
		t.Fatalf("precondition: Weeping Angel fixture not on battlefield with types: %+v", o)
	}
	Resolve(h, c, sa(t, "DB$ Animate | Defined$ Self | RemoveTypes$ Creature | Duration$ UntilYourNextTurn"))
	if len(h.continuous) != 1 {
		t.Fatalf("continuous = %d, want one type-removal effect: %+v", len(h.continuous), h.continuous)
	}
	ce := h.continuous[0]
	if ce.Layer != state.LType || len(ce.RemoveTypes) != 1 || ce.RemoveTypes[0] != "Creature" || len(ce.AddTypes) != 0 {
		t.Fatalf("type grant = %+v; want LType removal of Creature without additions", ce)
	}
}

func TestAnimateRemoveTypesBeforeAddedTypes(t *testing.T) {
	h, c := animateRemoveHost(t)
	Resolve(h, c, sa(t, "DB$ Animate | Defined$ Self | RemoveTypes$ Equipment | Types$ Creature Alien | Duration$ Permanent"))
	if len(h.continuous) != 1 {
		t.Fatalf("continuous = %+v, want type effect", h.continuous)
	}
	ce := h.continuous[0]
	if len(ce.RemoveTypes) != 1 || ce.RemoveTypes[0] != "Equipment" || len(ce.AddTypes) != 2 || ce.AddTypes[0] != "Creature" || ce.AddTypes[1] != "Alien" {
		t.Fatalf("effect = %+v, want strip then [Creature Alien] grant", ce)
	}
}
