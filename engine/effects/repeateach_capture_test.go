package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestRepeatEachRememberedExcludesTriggerCapture pins Soul of Emancipation's
// real ETB chain: the trigger's fired-event capture (the entering Soul) is not
// in Forge's host remembered list, so the loop visits only the two destroyed
// permanents and each controller gets exactly one Angel.
func TestRepeatEachRememberedExcludesTriggerCapture(t *testing.T) {
	card, sa := corpusSA(t, "Soul of Emancipation", "TrigDestroy")
	h := newHost(t, 2)
	h.g.Tokens = choiceCorpusRegistry(t).Tokens
	soul := h.g.AddObject(card, 0)
	hill := h.g.AddObject(mkCard(t, "Name:Hill Giant\nTypes:Creature Giant\nPT:3/3\nOracle:x\n"), 0)
	bear := h.g.AddObject(mkCard(t, "Name:Grizzly Bears\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 1)
	for _, o := range []*state.Object{soul, hill, bear} {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	}
	ctx := &Ctx{Source: soul.ID, Controller: 0, SVars: soul.Face().SVars,
		Remembered: []state.Target{{Obj: soul.ID}},
		Captured:   []state.Target{{Obj: soul.ID}},
		Targets:    []state.Target{{Obj: hill.ID}, {Obj: bear.ID}}, TargetsOffered: true}
	Resolve(h, ctx, sa)
	for _, id := range []state.ObjID{hill.ID, bear.ID} {
		if z := h.g.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("precondition: target %d is in %v, want destroyed", id, z)
		}
	}
	angels := func(p state.PlayerID) int {
		var n int
		for _, id := range h.g.Zone(state.ZBattlefield, p) {
			if o := h.g.Obj(id); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Angel Token" {
				n++
			}
		}
		return n
	}
	if got := angels(0); got != 1 {
		t.Fatalf("seat 0 has %d Angel tokens, want exactly 1 (the Soul must not be a RepeatEach subject)", got)
	}
	if got := angels(1); got != 1 {
		t.Fatalf("seat 1 has %d Angel tokens, want exactly 1", got)
	}
}
