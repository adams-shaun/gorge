package rules

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// attachTargetEffect stacks an inline Aura (with or without an AttachAILogic
// SVar) over a creature on seat 1's battlefield and returns the pending
// target decision.
func attachTargetEffect(t *testing.T, svar string) *decision.Decision {
	t.Helper()
	e := newSeats(t, 2)
	c := card(t, "Name:Test Pump Aura\nManaCost:G\nTypes:Enchantment Aura\nK:Enchant:Creature\n"+svar+"Oracle:x\n")
	o := e.G.AddObject(c, 0)
	e.emit(events.Event{Kind: events.PutOnStack, Obj: o.ID, Player: 0,
		From: state.ZLibrary, To: state.ZStack, Text: "attach logic"})
	o = e.G.Obj(o.ID)
	if o == nil || o.Zone != state.ZStack {
		t.Fatalf("fixture precondition: Aura is not on the stack: %+v", o)
	}
	sa := o.Face().SpellAbility()
	if sa == nil || sa.API != "Attach" {
		t.Fatalf("fixture precondition: spell ability = %+v, want an Attach SA", sa)
	}
	bear := card(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	bo := e.G.AddObject(bear, 1)
	e.emit(events.Event{Kind: events.MoveZone, Obj: bo.ID, From: state.ZLibrary, To: state.ZBattlefield})
	e.askTarget(0, o.ID, sa)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.TargetEffect == nil {
		t.Fatalf("fixture precondition: pending = %+v, want a KTarget with a TargetEffect", d)
	}
	return d
}

func TestTargetEffectPublishesAttachLogic(t *testing.T) {
	t.Parallel()
	d := attachTargetEffect(t, "SVar:AttachAILogic:Pump\n")
	if d.TargetEffect.API != "Attach" || d.TargetEffect.AttachLogic != "Pump" {
		t.Fatalf("TargetEffect = %+v, want API Attach with AttachLogic Pump", d.TargetEffect)
	}
	if got := attachTargetEffect(t, "").TargetEffect.AttachLogic; got != "" {
		t.Errorf("AttachLogic without the SVar = %q, want empty", got)
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(raw); strings.Contains(s, "AttachLogic") || strings.Contains(s, "attach_logic") {
		t.Errorf("the wire carries the server-side hint: %s", s)
	}
}
