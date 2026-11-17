package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEffectRemembersTriggeredAttackers pins an Effect's
// RememberObjects$ TriggeredAttackers (Love on the Battlefield): the plural
// AttackersDeclared spelling remembers the whole declared attacker group, the
// same group Defined$ TriggeredAttackers names.
func TestEffectRemembersTriggeredAttackers(t *testing.T) {
	card, sa := corpusSA(t, "Love on the Battlefield", "DBEffect")
	if sa.Params["RememberObjects"] != "TriggeredAttackers" {
		t.Fatalf("corpus selector changed: %q", sa.Params["RememberObjects"])
	}
	h := newHost(t, 2)
	srcObj := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: srcObj.ID, From: state.ZLibrary, To: state.ZBattlefield})
	var atk []state.ObjID
	for i := 0; i < 2; i++ {
		o := h.g.AddObject(mkCard(t, "Name:Atk\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
		if h.g.Obj(o.ID).Zone != state.ZBattlefield {
			t.Fatal("precondition: attacker must be on the battlefield")
		}
		atk = append(atk, o.ID)
	}
	ctx := &Ctx{Source: srcObj.ID, Controller: 0,
		Remembered: []state.Target{{Obj: atk[0]}, {Obj: atk[1]}}}
	got := effectRemembered(h, ctx, sa)
	if len(got) != 2 || got[0] != atk[0] || got[1] != atk[1] {
		t.Fatalf("remembered %v, want the declared attackers %v", got, atk)
	}
	if got := effectRemembered(h, &Ctx{Source: srcObj.ID, Controller: 0}, sa); len(got) != 0 {
		t.Fatalf("capture-free context remembered %v, want none", got)
	}
}
