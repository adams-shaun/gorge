package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestAttachParentTargetAttachesEveryObject: `Object$ ParentTarget` names the
// parent link's whole answered set (Thorin, Mountain-king's "attach any number
// of target Equipment you control to target creature you control"), so every
// object of the set is attached, in set order, and each is remembered under
// RememberAttached$ True. One object attaches one; an empty set attaches
// nothing and never falls back to the source.
func TestAttachParentTargetAttachesEveryObject(t *testing.T) {
	const line = "SP$ Attach | Object$ ParentTarget | Defined$ Remembered | RememberAttached$ True"
	setup := func(t *testing.T) (*fakeHost, *Ctx, map[string]state.ObjID) {
		h, c, ids := attachBoard(t)
		eq2 := h.g.AddObject(mkCard(t, "Name:Axe\nManaCost:1\nTypes:Artifact Equipment\nOracle:x\n"), 0)
		h.g.Obj(eq2.ID).Zone = state.ZBattlefield
		h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), eq2.ID))
		ids["eq2"] = eq2.ID
		// The source is a third object (Thorin's ETB), not an Equipment.
		src := h.g.AddObject(mkCard(t, "Name:King\nManaCost:2 R\nTypes:Creature Dwarf\nPT:3/3\nOracle:x\n"), 0)
		h.g.Obj(src.ID).Zone = state.ZBattlefield
		h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), src.ID))
		ids["src"] = src.ID
		c.Source = src.ID
		c.Remembered = []state.Target{{Obj: ids["bear"]}}
		return h, c, ids
	}
	attachs := func(h *fakeHost) []events.Event {
		var out []events.Event
		for _, ev := range h.log {
			if ev.Kind == events.Attach {
				out = append(out, ev)
			}
		}
		return out
	}

	t.Run("two", func(t *testing.T) {
		h, c, ids := setup(t)
		c.Targets = []state.Target{{Obj: ids["eq2"]}, {Obj: ids["eq"]}}
		Resolve(h, c, sa(t, line))
		got := attachs(h)
		if len(got) != 2 || got[0].Obj != ids["eq2"] || got[1].Obj != ids["eq"] {
			t.Fatalf("attachs = %+v, want eq2 then eq onto bear", got)
		}
		for _, ev := range got {
			if len(ev.IDs) != 1 || ev.IDs[0] != ids["bear"] {
				t.Fatalf("attach %+v not onto bear %d", ev, ids["bear"])
			}
		}
		if h.g.Obj(ids["eq"]).AttachedTo != ids["bear"] || h.g.Obj(ids["eq2"]).AttachedTo != ids["bear"] {
			t.Fatalf("both equipment must be attached to the bear")
		}
		var rem []state.ObjID
		for _, r := range c.Remembered {
			if r.Obj == ids["eq"] || r.Obj == ids["eq2"] {
				rem = append(rem, r.Obj)
			}
		}
		if len(rem) != 2 || rem[0] != ids["eq2"] || rem[1] != ids["eq"] {
			t.Fatalf("remembered attached = %v, want [eq2 eq]", rem)
		}
	})
	t.Run("one", func(t *testing.T) {
		h, c, ids := setup(t)
		c.Targets = []state.Target{{Obj: ids["eq"]}}
		Resolve(h, c, sa(t, line))
		if got := attachs(h); len(got) != 1 || got[0].Obj != ids["eq"] {
			t.Fatalf("attachs = %+v, want one eq onto bear", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		h, c, ids := setup(t)
		c.Targets = nil
		Resolve(h, c, sa(t, line))
		if got := attachs(h); len(got) != 0 {
			t.Fatalf("attachs = %+v, want none (never the source)", got)
		}
		if h.g.Obj(ids["src"]).AttachedTo != 0 {
			t.Fatalf("source attached to %d", h.g.Obj(ids["src"]).AttachedTo)
		}
	})
}
