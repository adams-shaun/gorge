package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// A controller read takes the live association, not the exile-gated object
// selector; it does not use the resolving ability's controller or card owner.
func TestDefinedImprintedControllerReadsPersistentPile(t *testing.T) {
	h := newHost(t, 3)
	src := h.g.AddObject(mkCard(t, "Name:Imprinter\nTypes:Enchantment\nOracle:x\n"), 0)
	linked := h.g.AddObject(mkCard(t, "Name:Linked\nTypes:Enchantment\nOracle:x\n"), 1)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZLibrary, To: state.ZBattlefield})
	h.Emit(events.Event{Kind: events.MoveZone, Obj: linked.ID, From: state.ZLibrary, To: state.ZBattlefield})
	h.Emit(events.Event{Kind: events.Imprint, Obj: src.ID, IDs: []state.ObjID{linked.ID}})
	// The imprinted card is controlled by a seat other than source or owner.
	h.Emit(events.Event{Kind: events.ControlChange, Obj: linked.ID, Player: 2})
	if o := h.g.Obj(linked.ID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 2 || o.Owner != 1 || o.Controller == h.g.Obj(src.ID).Controller {
		t.Fatalf("precondition: linked card = %+v; want battlefield owner 1, controller 2, distinct from source", o)
	}
	if ids := h.g.Obj(src.ID).Imprinted; len(ids) != 1 || ids[0] != linked.ID {
		t.Fatalf("precondition: imprint order = %v", ids)
	}
	c := &Ctx{Source: src.ID, Controller: 0}
	want := state.Target{Player: 2, IsPlayer: true}
	if got, ok := definedSpec(h, c, "ImprintedController"); !ok || len(got) != 1 || got[0] != want {
		t.Fatalf("definedSpec ImprintedController = %v ok=%v, want [%v]", got, ok, want)
	}
	if got := Defined(h, c, sa(t, "DB$ DealDamage | Defined$ ImprintedController")); len(got) != 1 || got[0] != want {
		t.Fatalf("Defined$ ImprintedController = %v, want [%v]", got, want)
	}
}

// TestEnchantersBaneImprintedControllerDamage drives the real corpus's
// DBSac -> TrigDamage -> DBCleanup chain after its TrigTarget has imprinted
// the targeted enchantment. The answered optional sacrifice is simulated by
// SacDone/SacPicks, the same resume fields the rules host supplies.
func TestEnchantersBaneImprintedControllerDamage(t *testing.T) {
	bane, sac := corpusSA(t, "Enchanter's Bane", "DBSac")
	if sac.API != "Sacrifice" || sac.Sub == nil || sac.Sub.API != "DealDamage" ||
		sac.Sub.Params["Defined"] != "ImprintedController" || sac.Sub.Params["DamageSource"] != "Imprinted" ||
		sac.Sub.Params["ConditionCheckSVar"] != "Y" || sac.Sub.Params["ConditionSVarCompare"] != "EQ0" {
		t.Fatalf("corpus chain changed: DBSac=%+v TrigDamage=%+v", sac, sac.Sub)
	}
	for _, tc := range []struct {
		name       string
		sacrifice  bool
		wantLife   int32
		wantDamage bool
	}{
		{"decline", false, 17, true},
		{"sacrifice", true, 20, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHost(t, 2)
			src := h.g.AddObject(bane, 0)
			linked := h.g.AddObject(mkCard(t, "Name:Opponent's Aura\nManaCost:2 W\nTypes:Enchantment\nOracle:x\n"), 1)
			for _, id := range []state.ObjID{src.ID, linked.ID} {
				h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
			}
			// TrigTarget's ImprintCards$ Targeted stores this association on
			// Bane; the real continuation starts at the optional sacrifice.
			h.Emit(events.Event{Kind: events.Imprint, Obj: src.ID, IDs: []state.ObjID{linked.ID}})
			if o := h.g.Obj(linked.ID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 || o.Face().ManaValue() != 3 {
				t.Fatalf("precondition: imprinted target = %+v, want battlefield enchantment controlled by 1 with MV 3", o)
			}
			if o := h.g.Obj(src.ID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 || len(o.Imprinted) != 1 || o.Imprinted[0] != linked.ID {
				t.Fatalf("precondition: Bane = %+v, want battlefield source with imprint", o)
			}
			if h.g.Players[0].Life != 20 || h.g.Players[1].Life != 20 || tc.wantLife == 20 && tc.wantDamage {
				t.Fatalf("precondition: life = %d/%d, expected result %d", h.g.Players[0].Life, h.g.Players[1].Life, tc.wantLife)
			}
			c := &Ctx{Source: src.ID, Controller: 0, Targets: []state.Target{{Obj: linked.ID}}, SVars: bane.Faces[0].SVars, SacDone: true}
			if tc.sacrifice {
				c.SacPicks = []state.ObjID{linked.ID}
			}
			Resolve(h, c, sac)
			damage := 0
			for _, e := range h.log {
				if e.Kind == events.Damage {
					damage++
					if e.Player != 1 || e.Amount != 3 {
						t.Fatalf("damage = %+v, want exactly 3 to enchantment's controller 1", e)
					}
				}
			}
			if got := h.g.Players[1].Life; got != tc.wantLife || (damage == 1) != tc.wantDamage {
				t.Fatalf("seat 1 life = %d damage events = %d, want life %d damage=%v; log=%+v", got, damage, tc.wantLife, tc.wantDamage, h.log)
			}
			if h.g.Players[0].Life != 20 {
				t.Fatalf("Bane's controller lost life: %d", h.g.Players[0].Life)
			}
			if tc.sacrifice && h.g.Obj(linked.ID).Zone != state.ZGraveyard {
				t.Fatalf("sacrifice did not move target: %s", h.g.Obj(linked.ID).Zone)
			}
		})
	}
}
