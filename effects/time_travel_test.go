package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestTimeTravelAsksPerObjectAndAppliesAnswer(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	card := mkCard(t, "Name:Suspended Card\nTypes:Sorcery\nOracle:x\n")
	o := h.g.AddObject(card, 0)
	o.Zone = state.ZExile
	h.g.SetZone(state.ZExile, 0, []state.ObjID{o.ID})
	o.CastFlags = state.FlagSuspend
	o.AddCounter("TIME", 2)
	src := h.g.AddObject(mkCard(t, "Name:Time Traveler\nTypes:Creature\nOracle:x\n"), 0)
	src.Zone = state.ZStack
	sa := sa(t, "DB$ TimeTravel")
	c := &Ctx{Source: src.ID, Controller: 0}

	// The suspended card and its positive TIME count are the preconditions for
	// the election; without either, this test could pass without exercising the
	// handler at all.
	if o.Counter("TIME") <= 0 || o.Zone != state.ZExile || o.CastFlags&state.FlagSuspend == 0 {
		t.Fatal("fixture is not a suspended TIME-counter card")
	}
	Resolve(h, c, sa)
	if h.asked == nil || h.asked.ResumeKind != "time_travel" {
		t.Fatalf("decision = %+v, want a TimeTravel election", h.asked)
	}
	if h.asked.Options[1].Kind != "time_travel_add" || h.asked.Options[2].Kind != "time_travel_remove" {
		t.Fatalf("options = %+v, want add/remove choices", h.asked.Options)
	}
	// Re-entry is what rules.resumeResolution supplies after the answer.
	rc := &Ctx{Source: src.ID, Controller: 0, TimeTravelChoice: "time_travel_add",
		TimeTravelDone: true, TimeTravelIndex: 0}
	Resolve(h, rc, sa)
	if got := h.g.Obj(o.ID).Counter("TIME"); got != 3 {
		t.Fatalf("TIME counter = %d, want 3 after add answer", got)
	}
	for _, ev := range h.log {
		if ev.Kind == events.Note && ev.Text == "unimplemented API TimeTravel" {
			t.Fatal("TimeTravel fell through the unimplemented handler")
		}
	}
}

func TestTimeTravelKeepsObjectIdentityWhenRemovalShrinksEligibility(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	src := h.g.AddObject(mkCard(t, "Name:Time Traveler\nTypes:Creature\nOracle:x\n"), 0)
	src.Zone = state.ZStack
	ids := make([]state.ObjID, 0, 3)
	for _, name := range []string{"A", "B", "C"} {
		o := h.g.AddObject(mkCard(t, "Name:"+name+"\nTypes:Creature\nOracle:x\n"), 0)
		o.Zone = state.ZBattlefield
		o.AddCounter("TIME", 1)
		ids = append(ids, o.ID)
	}
	h.g.SetZone(state.ZBattlefield, 0, ids)
	sa := sa(t, "DB$ TimeTravel")
	Resolve(h, &Ctx{Source: src.ID, Controller: 0}, sa)
	if h.asked == nil || h.asked.Options[0].Obj != ids[0] {
		t.Fatalf("first ask = %+v, want object %d", h.asked, ids[0])
	}
	snapshot := append([]state.ObjID(nil), ids...)
	Resolve(h, &Ctx{Source: src.ID, Controller: 0, TimeTravelChoice: "time_travel_remove", TimeTravelDone: true, TimeTravelIndex: 0, TimeTravelObjects: snapshot}, sa)
	if h.asked == nil || h.asked.Options[0].Obj != ids[1] {
		t.Fatalf("second ask = %+v, want object %d after removing first", h.asked, ids[1])
	}
	Resolve(h, &Ctx{Source: src.ID, Controller: 0, TimeTravelChoice: "time_travel_remove", TimeTravelDone: true, TimeTravelIndex: 1, TimeTravelObjects: snapshot}, sa)
	if h.g.Obj(ids[0]).Counter("TIME") != 0 || h.g.Obj(ids[1]).Counter("TIME") != 0 || h.g.Obj(ids[2]).Counter("TIME") != 1 {
		t.Fatalf("TIME counters = %d,%d,%d; answers drifted after removal", h.g.Obj(ids[0]).Counter("TIME"), h.g.Obj(ids[1]).Counter("TIME"), h.g.Obj(ids[2]).Counter("TIME"))
	}
}

func TestTimeTravelOffersZeroCounterSuspendedCard(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	o := h.g.AddObject(mkCard(t, "Name:Suspended Zero\nTypes:Sorcery\nOracle:x\n"), 0)
	o.Zone = state.ZExile
	o.CastFlags = state.FlagSuspend
	h.g.SetZone(state.ZExile, 0, []state.ObjID{o.ID})
	src := h.g.AddObject(mkCard(t, "Name:Time Traveler\nTypes:Creature\nOracle:x\n"), 0)
	src.Zone = state.ZStack
	Resolve(h, &Ctx{Source: src.ID, Controller: 0}, sa(t, "DB$ TimeTravel"))
	if h.asked == nil || h.asked.Options[0].Obj != o.ID {
		t.Fatalf("decision = %+v, want suspended zero-counter card %d", h.asked, o.ID)
	}
	Resolve(h, &Ctx{Source: src.ID, Controller: 0, TimeTravelChoice: "time_travel_add", TimeTravelDone: true, TimeTravelIndex: 0, TimeTravelObjects: []state.ObjID{o.ID}}, sa(t, "DB$ TimeTravel"))
	if got := h.g.Obj(o.ID).Counter("TIME"); got != 1 {
		t.Fatalf("TIME counter = %d, want 1 after adding to suspended zero-counter card", got)
	}
}
