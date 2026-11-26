package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A phased-out permanent is treated as though it does not exist (CR
// 702.26b), so SacrificeAll over Defined$ Enchanted sacrifices nothing once
// the host has phased out (Parasitic Implant's upkeep trigger resolving after
// Slip Out the Back phased its creature out with the Aura). The same host
// phased in is sacrificed.
func TestSacrificeAllDefinedSkipsPhasedOut(t *testing.T) {
	h := newHost(t, 2)
	bear := h.g.AddObject(mkCard(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 1)
	aura := h.g.AddObject(mkCard(t, "Name:Implant\nManaCost:3 B\nTypes:Enchantment Aura\nOracle:x\n"), 0)
	for _, o := range []*state.Object{bear, aura} {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	}
	h.Emit(events.Event{Kind: events.Attach, Obj: aura.ID, IDs: []state.ObjID{bear.ID}})
	body := sa(t, "DB$ SacrificeAll | Defined$ Enchanted")

	h.Emit(events.Event{Kind: events.PhaseOut, Obj: bear.ID, Amount: 1})
	if !h.g.Obj(bear.ID).PhasedOut {
		t.Fatal("precondition: the host phased out")
	}
	effSacrificeAll(h, &Ctx{Source: aura.ID, Controller: 0}, body)
	if o := h.g.Obj(bear.ID); o.Zone != state.ZBattlefield {
		t.Fatalf("phased-out host zone = %v, want it left on the battlefield", o.Zone)
	}

	h.Emit(events.Event{Kind: events.PhaseOut, Obj: bear.ID, Amount: -1})
	effSacrificeAll(h, &Ctx{Source: aura.ID, Controller: 0}, body)
	if o := h.g.Obj(bear.ID); o.Zone != state.ZGraveyard {
		t.Fatalf("phased-in host zone = %v, want the graveyard", o.Zone)
	}
}
