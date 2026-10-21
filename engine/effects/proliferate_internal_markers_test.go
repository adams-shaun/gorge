package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestProliferateIgnoresInternalMarkers pins CR 701.27a against the engine's
// status markers: "Shield" (the this-turn regeneration shield) and
// "Deathtouched" ride Object.Counters but are not counters a card can see
// (state.InternalCounterMarker), so proliferate must neither offer a permanent
// whose only "counter" is a marker nor give a chosen recipient another one.
func TestProliferateIgnoresInternalMarkers(t *testing.T) {
	t.Run("shield-only permanent is not offered", func(t *testing.T) {
		h := &askHost{}
		h.g = state.NewGame(names(2))
		shielded := h.g.AddObject(mkCard(t, "Name:Shielded\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
		shielded.Zone = state.ZBattlefield
		shielded.AddCounter("Shield", 1)
		h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{shielded.ID})
		if shielded.Counter("Shield") != 1 {
			t.Fatal("precondition: the regeneration shield was not recorded on the object")
		}
		c := &Ctx{Source: shielded.ID, Controller: 0}
		Resolve(h, c, proliferateSA(t, ""))
		if h.asked != nil {
			t.Fatalf("a permanent whose only counter is a shield was offered: %+v", h.asked)
		}
	})

	t.Run("real counter is added, shield is not duplicated", func(t *testing.T) {
		h := &fakeHost{}
		h.g = state.NewGame(names(2))
		creature := h.g.AddObject(mkCard(t, "Name:Creature\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
		creature.Zone = state.ZBattlefield
		creature.AddCounter("P1P1", 1)
		creature.AddCounter("Shield", 1)
		h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{creature.ID})
		if creature.Counter("P1P1") != 1 || creature.Counter("Shield") != 1 {
			t.Fatal("precondition: expected one +1/+1 counter and one shield")
		}
		c := &Ctx{Source: creature.ID, Controller: 0}
		Resolve(h, c, proliferateSA(t, ""))
		if got := creature.Counter("P1P1"); got != 2 {
			t.Fatalf("P1P1 = %d, want 2 (proliferate adds one real counter)", got)
		}
		if got := creature.Counter("Shield"); got != 1 {
			t.Fatalf("Shield = %d, want 1 (proliferate must not add a second shield)", got)
		}
		for _, ev := range h.log {
			if ev.Kind == events.CounterChange && ev.Counter == "Shield" {
				t.Fatalf("proliferate emitted a CounterChange for the internal shield marker: %+v", ev)
			}
		}
	})
}
