package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestGrantTriggerPushRebuildsAGrantedEvolve: a granted Evolve (CR 702.100a,
// 613.1f) has no SVar table to resolve its body from, so Apply rebuilds the
// printed expansion's +1/+1 counter body from the payload alone, with the
// recipient as Source and the grantor recorded.
func TestGrantTriggerPushRebuildsAGrantedEvolve(t *testing.T) {
	g, l := twoPlayer(t)
	grantor := g.AddObject(bearCard(), 0)
	grantor.Zone = state.ZBattlefield
	recipient := g.AddObject(bearCard(), 0)
	recipient.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{grantor.ID, recipient.ID})
	if g.Obj(recipient.ID).Face() == nil || ResolveSVarAcrossFaces(grantor, "__kwEvolveGranted") != nil {
		t.Fatal("precondition: the payload must not resolve as a printed SVar")
	}

	Emit(g, l, Event{Kind: GrantTriggerPush, Player: 0, Obj: recipient.ID, Counter: "__kwEvolveGranted",
		Amount: int32(grantor.ID), Text: "granted trigger"})

	if len(g.Stack) != 1 {
		t.Fatalf("stack = %v, want the one ability object", g.Stack)
	}
	a := g.Obj(g.Stack[0])
	if a.Ability == nil || a.Ability.API != "PutCounter" || a.Ability.Params["Defined"] != "Self" ||
		a.Ability.Params["CounterType"] != "P1P1" || a.Ability.Params["CounterNum"] != "1" {
		t.Fatalf("Ability = %+v, want the evolve PutCounter body", a.Ability)
	}
	if a.Source != recipient.ID || a.GrantedBy != grantor.ID {
		t.Fatalf("Source/GrantedBy = %d/%d, want %d/%d", a.Source, a.GrantedBy, recipient.ID, grantor.ID)
	}
}
