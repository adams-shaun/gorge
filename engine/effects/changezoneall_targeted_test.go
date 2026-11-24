package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Tawnos's Coffin's DBExile sweep, "ChangeZoneAll | ChangeType$
// Targeted.Self,Aura.AttachedTo Card.Self": Forge's filterListByType makes the
// target the relative source of every entry, so the sweep exiles the target
// and the Auras attached to IT (CR 701.13a), not Auras attached to the Coffin
// and not an Aura on another creature.
func TestChangeZoneAllTargetedSweepsTargetAndItsAuras(t *testing.T) {
	coffin, sweep := corpusSA(t, "Tawnos's Coffin", "DBExile")
	if got := sweep.Params["ChangeType"]; sweep.API != "ChangeZoneAll" || got != "Targeted.Self,Aura.AttachedTo Card.Self" {
		t.Fatalf("Tawnos's Coffin sweep fixture changed: %s %q", sweep.API, got)
	}
	h := newHost(t, 2)
	src := h.g.AddObject(coffin, 0)
	creature := "Types:Creature\nPT:2/2\nOracle:x\n"
	aura := "Types:Enchantment Aura\nK:Enchant creature\nOracle:x\n"
	target := h.g.AddObject(mkCard(t, "Name:Target\n"+creature), 1)
	bystander := h.g.AddObject(mkCard(t, "Name:Bystander\n"+creature), 1)
	onTarget := h.g.AddObject(mkCard(t, "Name:On Target\n"+aura), 0)
	onBystander := h.g.AddObject(mkCard(t, "Name:On Bystander\n"+aura), 1)
	for _, o := range []*state.Object{src, target, bystander, onTarget, onBystander} {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	}
	h.Emit(events.Event{Kind: events.Attach, Obj: onTarget.ID, IDs: []state.ObjID{target.ID}})
	h.Emit(events.Event{Kind: events.Attach, Obj: onBystander.ID, IDs: []state.ObjID{bystander.ID}})
	if h.g.Obj(onTarget.ID).AttachedTo != target.ID || h.g.Obj(onBystander.ID).AttachedTo != bystander.ID {
		t.Fatalf("precondition: Auras not attached")
	}

	effChangeZoneAll(h, &Ctx{Source: src.ID, Controller: 0, Targets: []state.Target{{Obj: target.ID}}}, sweep)

	for _, want := range []struct {
		id   state.ObjID
		zone state.Zone
	}{{target.ID, state.ZExile}, {onTarget.ID, state.ZExile}, {bystander.ID, state.ZBattlefield},
		{onBystander.ID, state.ZBattlefield}, {src.ID, state.ZBattlefield}} {
		o := h.g.Obj(want.id)
		if o == nil {
			t.Fatalf("object %d vanished", want.id)
		}
		if o.Zone != want.zone {
			t.Errorf("object %d zone = %v, want %v", want.id, o.Zone, want.zone)
		}
	}
}
