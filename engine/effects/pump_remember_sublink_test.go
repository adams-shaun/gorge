package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPumpSubLinkRemembersItsOwnThisTargetedCard pins Graceful Takedown's
// sub-link Pump (DBPumpBis: its own ValidTgts$ plus RememberObjects$
// ThisTargetedCard). On a sub link Ctx.Targets is the chain ROOT's list and
// the sub's own answer arrives in Ctx.PickedTargets (CR 115.1, 608.2b), so the
// remember must read the latter, not re-remember the root's pick.
func TestPumpSubLinkRemembersItsOwnThisTargetedCard(t *testing.T) {
	_, sa := corpusSA(t, "Graceful Takedown", "DBPumpBis")
	if sa.Params["RememberObjects"] != "ThisTargetedCard" || sa.Params["ValidTgts"] == "" {
		t.Fatalf("corpus shape changed: %v", sa.Params)
	}
	h := newHost(t, 2)
	battlefield := func(name string) state.ObjID {
		t.Helper()
		o := h.g.AddObject(mkCard(t, "Name:"+name+"\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
		if got := h.g.Obj(o.ID); got == nil || got.Zone != state.ZBattlefield {
			t.Fatalf("precondition: %s not on the battlefield: %+v", name, got)
		}
		return o.ID
	}
	a, b := battlefield("A"), battlefield("B")
	if a == b {
		t.Fatal("precondition: root pick and sub pick must differ")
	}
	remember := func(targets, picked []state.Target) []state.Target {
		c := &Ctx{Source: a, Controller: 0, Targets: targets, PickedTargets: picked}
		rememberDefinedObjects(h, c, RefOf("ThisTargetedCard"), "Pump")
		return c.Remembered
	}
	only := func(t *testing.T, got []state.Target, want ...state.ObjID) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("remembered %v, want %v", got, want)
		}
		for i := range want {
			if got[i].Obj != want[i] {
				t.Fatalf("remembered %v, want %v", got, want)
			}
		}
	}

	t.Run("own pick", func(t *testing.T) {
		only(t, remember([]state.Target{{Obj: a}}, []state.Target{{Obj: b}}), b)
	})
	t.Run("root link (no pre-ask)", func(t *testing.T) {
		only(t, remember([]state.Target{{Obj: a}}, nil), a)
	})
	t.Run("up to one answered with none", func(t *testing.T) {
		only(t, remember([]state.Target{{Obj: a}}, []state.Target{}))
	})
}
