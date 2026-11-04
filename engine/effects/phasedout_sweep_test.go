package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPhasedOutPermanentIsOutsideEverySweep pins CR 702.26b for the
// battlefield sweeps: a phased-out creature gets no counter from "put a
// +1/+1 counter on each creature you control" and is not counted by a
// Count$Valid over the battlefield; the phased-in creature beside it is. The
// phasing toggle's AllValid$ pool still reaches it, so it can phase back in.
func TestPhasedOutPermanentIsOutsideEverySweep(t *testing.T) {
	h := newHost(t, 2)
	bear := func() state.ObjID {
		o := h.g.AddObject(mkCard(t, "Name:Grove Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
		o.Zone = state.ZBattlefield
		return o.ID
	}
	in, out := bear(), bear()
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{in, out})
	h.Emit(events.Event{Kind: events.PhaseOut, Obj: out, Amount: 1})
	if !h.g.Obj(out).PhasedOut {
		t.Fatal("precondition: the second bear was not phased out")
	}
	c := &Ctx{Controller: 0}
	if n, ok := EvalCountOK(h, c, "Count$Valid Creature.YouCtrl"); !ok || n != 1 {
		t.Fatalf("Count$Valid Creature.YouCtrl = (%d, %v), want (1, true)", n, ok)
	}
	Resolve(h, c, sa(t, "DB$ PutCounterAll | ValidCards$ Creature.YouCtrl | CounterType$ P1P1 | CounterNum$ 1"))
	if got := h.g.Obj(in).Counter("P1P1"); got != 1 {
		t.Fatalf("the phased-in bear has %d +1/+1 counters, want 1", got)
	}
	if got := h.g.Obj(out).Counter("P1P1"); got != 0 {
		t.Fatalf("CR 702.26b: the phased-out bear got %d +1/+1 counters", got)
	}
	Resolve(h, c, sa(t, "DB$ Phases | AllValid$ Creature.YouCtrl | PhaseInOrOut$ True"))
	if h.g.Obj(out).PhasedOut || !h.g.Obj(in).PhasedOut {
		t.Fatalf("the toggle did not swap the bears: in phased out %v, out phased out %v", h.g.Obj(in).PhasedOut, h.g.Obj(out).PhasedOut)
	}
}
