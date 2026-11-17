package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 702.26b: a phased-out Aura (Felidar Umbra phased out indirectly with its
// bearer) is treated as though it does not exist, so its Attach ability
// resolving can't move it.
func TestAttachDoesNotMoveAPhasedOutSource(t *testing.T) {
	h, c, ids := attachBoard(t)
	h.g.Obj(ids["eq"]).PhasedOut = true
	h.g.Obj(ids["eq"]).PhasedOutIndirect = true
	if o := h.g.Obj(ids["eq"]); !o.PhasedOut || o.Zone != state.ZBattlefield || h.g.Obj(ids["bear"]).PhasedOut {
		t.Fatal("precondition: the source is phased out on the battlefield and the destination is phased in")
	}
	c.Remembered = []state.Target{{Obj: ids["bear"]}}
	Resolve(h, c, sa(t, "SP$ Attach | Object$ Self | Defined$ Remembered"))

	for _, ev := range h.log {
		if ev.Kind == events.Attach {
			t.Fatalf("unexpected Attach %+v of a phased-out permanent", ev)
		}
	}
	if h.g.Obj(ids["eq"]).AttachedTo != 0 {
		t.Fatalf("phased-out source attached to %d", h.g.Obj(ids["eq"]).AttachedTo)
	}
}
