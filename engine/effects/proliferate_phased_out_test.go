package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestProliferateSkipsPhasedOutPermanent pins CR 702.26b: a phased-out
// permanent is treated as though it does not exist, so proliferate offers
// neither it nor, when it is the only carrier, any ask at all.
func TestProliferateSkipsPhasedOutPermanent(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	cadets := h.g.AddObject(mkCard(t, "Name:Cadets\nTypes:Creature\nPT:2/1\nOracle:x\n"), 0)
	cadets.Zone = state.ZBattlefield
	cadets.AddCounter("P1P1", 2)
	cadets.PhasedOut = true
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{cadets.ID})
	if !cadets.PhasedOut || cadets.Counter("P1P1") != 2 {
		t.Fatal("precondition: a phased-out permanent holding two +1/+1 counters")
	}
	Resolve(h, &Ctx{Source: cadets.ID, Controller: 0}, proliferateSA(t, ""))
	if h.asked != nil {
		t.Fatalf("a phased-out permanent was offered to proliferate: %+v", h.asked)
	}
	if got := cadets.Counter("P1P1"); got != 2 {
		t.Fatalf("P1P1 = %d, want 2 (a phased-out permanent is not a recipient)", got)
	}
}
