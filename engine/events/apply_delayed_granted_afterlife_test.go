package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestGrantTriggerPushRebuildsAGrantedAfterlife: a granted Afterlife N (CR
// 702.135a) rebuilds the printed expansion's Spirit token body from the
// "__kwAfterlifeGranted:N" payload alone, N riding as TokenAmount$. A bare
// payload, a zero count and a non-literal count rebuild nothing.
func TestGrantTriggerPushRebuildsAGrantedAfterlife(t *testing.T) {
	g, l := twoPlayer(t)
	recipient := g.AddObject(bearCard(), 0)
	recipient.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{recipient.ID})
	if ResolveSVarAcrossFaces(recipient, "__kwAfterlifeGranted:3") != nil {
		t.Fatal("precondition: the payload must not resolve as a printed SVar")
	}
	for _, bad := range []string{"__kwAfterlifeGranted", "__kwAfterlifeGranted:", "__kwAfterlifeGranted:0", "__kwAfterlifeGranted:X", "__kwAfterlifeGranted:03"} {
		Emit(g, l, Event{Kind: GrantTriggerPush, Player: 0, Obj: recipient.ID, Counter: bad, Text: "granted trigger"})
	}
	if len(g.Stack) != 0 {
		t.Fatalf("stack = %v after the malformed payloads, want empty", g.Stack)
	}

	Emit(g, l, Event{Kind: GrantTriggerPush, Player: 0, Obj: recipient.ID, Counter: "__kwAfterlifeGranted:3", Text: "granted trigger"})

	if len(g.Stack) != 1 {
		t.Fatalf("stack = %v, want the one ability object", g.Stack)
	}
	a := g.Obj(g.Stack[0])
	if a.Ability == nil || a.Ability.API != "Token" || a.Ability.Params["TokenScript"] != "wb_1_1_spirit_flying" ||
		a.Ability.Params["TokenAmount"] != "3" {
		t.Fatalf("Ability = %+v, want the afterlife Token body with TokenAmount 3", a.Ability)
	}
	if a.Source != recipient.ID {
		t.Fatalf("Source = %d, want the recipient %d", a.Source, recipient.ID)
	}
}
