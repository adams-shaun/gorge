package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// selfSacrificeStolen puts a source on seat 0's battlefield and hands it to
// seat 1 by a ControlChange event, with the ability's context still seat 0's.
func selfSacrificeStolen(t *testing.T) (*fakeHost, *Ctx, state.ObjID) {
	t.Helper()
	h := newHost(t, 2)
	id := putBattlefield(h, 0, "Name:Land\nTypes:Land\nOracle:x\n")
	c := NewCtxPtr(id, 0, CtxInit{})
	h.Emit(events.Event{Kind: events.ControlChange, Obj: id, Player: 1})
	if o := h.g.Obj(id); o.Controller != 1 || o.Zone != state.ZBattlefield || c.Controller != 0 {
		t.Fatalf("precondition: source must be seat 1's on the battlefield under a seat 0 ability: %+v ctl=%d", o, c.Controller)
	}
	return h, c, id
}

// TestSelfSacrificeNeedsTheAbilityController: CR 701.21a, "you" is the
// ability's controller (CR 608.2c); after the source changed control only
// that player's permanents can be sacrificed, so the source stays put and
// nobody is asked (optional) or forced (mandatory).
func TestSelfSacrificeNeedsTheAbilityController(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra map[string]string
	}{
		{"optional", map[string]string{"SacValid": "Self", "Optional": "True", "RememberSacrificed": "True"}},
		{"mandatory", map[string]string{"SacValid": "Self", "RememberSacrificed": "True"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, c, id := selfSacrificeStolen(t)
			effSacrifice(h, c, sacrificeParams(tc.extra))
			if h.askCount != 0 {
				t.Fatalf("asked %d decisions (last %+v), want none", h.askCount, h.lastAsk)
			}
			if z := h.g.Obj(id).Zone; z != state.ZBattlefield {
				t.Fatalf("source zone = %v, want battlefield", z)
			}
			if len(c.Remembered) != 0 {
				t.Fatalf("remembered %v, want nothing", c.Remembered)
			}
		})
	}
}

// TestSelfSacrificeKeepsTheControllersOwnSource guards the other side: an
// unstolen source is still offered to its controller.
func TestSelfSacrificeKeepsTheControllersOwnSource(t *testing.T) {
	h := newHost(t, 2)
	id := putBattlefield(h, 0, "Name:Land\nTypes:Land\nOracle:x\n")
	c := NewCtxPtr(id, 0, CtxInit{})
	effSacrifice(h, c, sacrificeParams(map[string]string{"SacValid": "Self", "Optional": "True"}))
	if h.askCount != 1 || h.lastAsk == nil || h.lastAsk.Player != 0 {
		t.Fatalf("asks=%d last=%+v, want one ask of seat 0", h.askCount, h.lastAsk)
	}
}
