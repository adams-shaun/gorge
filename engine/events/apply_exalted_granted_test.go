package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestKeywordTriggerPushRebuildsAGrantedExaltedInstance: a granted Exalted
// instance (CR 702.83a-b) has no printed trigger line, so Apply rebuilds the
// printed expansion's Pump of the triggering attacker from the payload alone,
// with the declaration's attacker and the defending player as Remembered.
func TestKeywordTriggerPushRebuildsAGrantedExaltedInstance(t *testing.T) {
	g, l := twoPlayer(t)
	holder := g.AddObject(bearCard(), 0)
	holder.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{holder.ID})
	attacker := state.ObjID(7)

	Emit(g, l, Event{Kind: KeywordTriggerPush, Player: 0, Obj: holder.ID, Counter: "__kwExaltedGranted",
		IDs: []state.ObjID{attacker, state.PlayerRef(1)}, Text: "exalted ability"})

	if len(g.Stack) != 1 {
		t.Fatalf("stack = %v, want the one ability object", g.Stack)
	}
	a := g.Obj(g.Stack[0])
	if a.Ability == nil || a.Ability.API != "Pump" || a.Ability.Params["Defined"] != "TriggeredAttacker" ||
		a.Ability.Params["NumAtt"] != "+1" || a.Ability.Params["NumDef"] != "+1" {
		t.Fatalf("Ability = %+v, want the exalted Pump of the triggered attacker", a.Ability)
	}
	if a.Source != holder.ID {
		t.Fatalf("Source = %d, want the holder %d", a.Source, holder.ID)
	}
	if len(a.Remembered) != 2 || a.Remembered[0].Obj != attacker || !a.Remembered[1].IsPlayer || a.Remembered[1].Player != 1 {
		t.Fatalf("Remembered = %v, want the attacker and the defending player", a.Remembered)
	}
}
