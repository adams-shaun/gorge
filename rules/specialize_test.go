package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestSpecializeGateBindsAtSixControlledLands(t *testing.T) {
	c, diags := cards.ParseBytes("gate.txt", []byte(`Name:Front
AlternateMode:Specialize
Types:Creature
K:Specialize:0::IsPresent$ Land.YouCtrl | PresentCompare$ GE6
SPECIALIZE:WHITE
Name:White Form
Types:Creature
`))
	if len(diags) != 0 {
		t.Fatalf("parse diags: %+v", diags)
	}
	land, _ := cards.ParseBytes("land.txt", []byte("Name:Test Land\nTypes:Land\n"))
	e := handEngine(t, c)
	front := e.G.AddObject(c, 0)
	front.Zone = state.ZBattlefield
	landIDs := make([]state.ObjID, 0, 6)
	for i := 0; i < 6; i++ {
		o := e.G.AddObject(land, 0)
		o.Zone = state.ZBattlefield
		landIDs = append(landIDs, o.ID)
	}
	e.G.SetZone(state.ZBattlefield, 0, append([]state.ObjID{front.ID}, landIDs[:5]...))
	if _, ok := e.specializeLegal(0, front.ID, 1); ok {
		t.Fatal("gate held with only five controlled lands")
	}
	e.G.SetZone(state.ZBattlefield, 0, append([]state.ObjID{front.ID}, landIDs...))
	params := map[string]string{"IsPresent": "Land.YouCtrl", "PresentCompare": "GE6"}
	if got := e.countStaticPresent(staticView{Source: front.ID, Controller: 0, Params: params}, params["IsPresent"]); got != 6 {
		t.Fatalf("gate precondition: countStaticPresent=%d, want 6", got)
	}
	if _, ok := e.specializeLegal(0, front.ID, 1); !ok {
		t.Fatal("gate suppressed with six controlled lands")
	}
}

func TestSpecializeSpecialActionChangesFaceAndMatchesTrigger(t *testing.T) {
	c, diags := cards.ParseBytes("specialize.txt", []byte(`Name:Front
AlternateMode:Specialize
Types:Creature Druid
K:Specialize:0
SPECIALIZE:WHITE
Name:White Form
ManaCost:W
Types:Creature
T:Mode$ Specializes | Execute$ Trig
SVar:Trig:DB$ PutCounter | CounterType$ P1P1 | CounterNum$ 1 | Defined$ Self
`))
	if len(diags) != 0 {
		t.Fatalf("parse diags: %+v", diags)
	}
	e := handEngine(t)
	for p := state.PlayerID(0); p < 2; p++ {
		e.G.SetZone(state.ZBattlefield, p, nil)
	}
	o := e.G.AddObject(c, 0)
	o.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{o.ID})
	if o.FaceIdx != 0 || o.Zone != state.ZBattlefield {
		t.Fatalf("bad setup: face=%d zone=%s", o.FaceIdx, o.Zone)
	}
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("priority decision missing: %+v", d)
	}
	opts := d.Options
	idx := -1
	for _, op := range opts {
		if op.Kind == "specialize" && op.Obj == o.ID && op.Mode == "1" {
			idx = op.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no face-1 specialize option: %+v", opts)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{idx}}); err != nil {
		t.Fatal(err)
	}
	if o.FaceIdx != 1 {
		t.Fatalf("FaceIdx=%d, want 1", o.FaceIdx)
	}
	found := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Specialize && ev.Obj == o.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("specialize transition was not event sourced")
	}
	tr := o.Face().Triggers[0]
	ev := events.Event{Kind: events.Specialize, Obj: o.ID}
	if !trigMatchers["Specializes"](e, tr, o.ID, ev, nil) {
		t.Fatal("Mode$ Specializes matcher did not match its own object's transition")
	}
}
