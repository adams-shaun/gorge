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
