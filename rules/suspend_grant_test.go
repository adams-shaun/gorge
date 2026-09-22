package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestDynamicSuspendGrantIsDerivedAndFilterable(t *testing.T) {
	e := handEngine(t)
	c := card(t, "Name:Grantable\nManaCost:2 U\nTypes:Creature\nPT:2/2\nOracle:x\n")
	o := e.G.AddObject(c, 0)
	o.Zone = state.ZExile
	e.G.SetZone(state.ZExile, 0, []state.ObjID{o.ID})
	e.emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: "TIME", Amount: 2})
	if e.HasKeyword(o.ID, "Suspend") {
		t.Fatal("test setup unexpectedly gave the card suspend")
	}
	e.emit(events.Event{Kind: events.AlterAttribute, Obj: o.ID, Text: "Suspend", Amount: 1})
	if !o.SuspendGranted || !e.HasKeyword(o.ID, "Suspend") {
		t.Fatalf("granted suspend was not derived: granted=%v keywords=%v", o.SuspendGranted, e.Derived(o.ID).Keywords)
	}
	if !effects.MatchesSpecFrom(e.G, "Card.withSuspend", o.ID, 0, o.ID) {
		t.Fatal("Card.withSuspend did not see the granted keyword")
	}
	if !effects.MatchesSpecFrom(e.G, "Card.suspended", o.ID, 0, o.ID) {
		t.Fatal("Card.suspended did not see the exiled card with TIME")
	}
	if effects.MatchesSpecFrom(e.G, "Card.withoutSuspend", o.ID, 0, o.ID) {
		t.Fatal("Card.withoutSuspend matched a card with granted suspend")
	}
}

func TestFaceOfBoeSuspendCostUsesChosenCardKeyword(t *testing.T) {
	f := card(t, "Name:Suspended\nManaCost:5 R\nTypes:Sorcery\nK:Suspend:3:1 R\nOracle:x\n").Faces[0]
	got, ok := pricePlayCost(f, "SuspendCost")
	if !ok || got.Generic != 1 || got.Colored[state.MR] != 1 {
		t.Fatalf("SuspendCost = %+v, %v; want {1}{R}", got, ok)
	}
}
