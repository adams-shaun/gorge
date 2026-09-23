package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestDelayedTriggerSpellCastStoresValidPlayer(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Source: 1, Controller: 0}, sa(t, "DB$ DelayedTrigger | Mode$ SpellCast | ValidPlayer$ Opponent | Execute$ X"))
	if len(h.log) != 1 {
		t.Fatalf("registration log = %+v, want one event", h.log)
	}
	if !strings.Contains(h.log[0].Text, "ValidPlayer$ Opponent") {
		t.Fatalf("registration text = %q, want ValidPlayer$ Opponent", h.log[0].Text)
	}
}

func TestDelayedTriggerBodyStoresInterveningConditions(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Source: 1, Controller: 0}, sa(t, "DB$ DelayedTrigger | Mode$ ChangesController | IsPresent$ Card.IsCommander+YouCtrl | PresentDefined$ You | PresentCompare$ GE2 | PresentZone$ Battlefield | Execute$ X"))
	if len(h.log) != 1 {
		t.Fatalf("registration log = %+v", h.log)
	}
	for _, clause := range []string{"IsPresent$", "PresentDefined$", "PresentCompare$", "PresentZone$"} {
		if !strings.Contains(h.log[0].Text, clause) {
			t.Errorf("registration lost %s: %q", clause, h.log[0].Text)
		}
	}
}

func TestDelayedTriggerRememberObjectsTargetedUsesTargets(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Source: 1, Controller: 0,
		Remembered: []state.Target{{Obj: 7}},
		Targets:    []state.Target{{Obj: 42}}}
	if c.Remembered[0].Obj == c.Targets[0].Obj {
		t.Fatal("precondition: chain and targeted captures must differ")
	}
	Resolve(h, c, sa(t, "DB$ DelayedTrigger | Mode$ DamageDone | RememberObjects$ Targeted | Execute$ X"))
	if len(h.log) != 1 || h.log[0].Kind != events.DelayedRegister {
		t.Fatalf("log = %+v, want one registration", h.log)
	}
	if len(h.log[0].IDs) != 1 || h.log[0].IDs[0] != 42 {
		t.Fatalf("registration IDs = %v, want targeted object [42]", h.log[0].IDs)
	}
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "RememberObjects$") {
			t.Fatalf("Targeted capture still degraded: %q", ev.Text)
		}
	}
}
