package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestFoldDelayedRegisterStampZoneGen pins the "|RS=" remembered-stamp
// spelling's ZoneGen slot (state.DelayedTrigger.RememberedStamps, CR 400.7): a
// three-field pair folds the logged ZoneGen; a legacy two-field pair, and a
// pair whose third field is malformed, take the object's live ZoneGen (not 0,
// which a never-moved object holds), without a panic.
func TestFoldDelayedRegisterStampZoneGen(t *testing.T) {
	fold := func(text string) (state.TargetStamp, uint32) {
		t.Helper()
		g := state.NewGame([]string{"Ann", "Bob"})
		card, _ := cards.ParseBytes("c.txt", []byte("Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"))
		o := g.AddObject(card, 0)
		id := o.ID
		g.SetZone(state.ZBattlefield, 0, []state.ObjID{id})
		o.Zone = state.ZBattlefield
		o.ZoneGen = 7
		Apply(g, Event{Kind: DelayedRegister, Obj: id, Player: 0, Step: state.StepEndCombat,
			Counter: "Body", IDs: []state.ObjID{id}, Text: text})
		if len(g.Delayed) != 1 || len(g.Delayed[0].RememberedStamps) != 1 {
			t.Fatalf("Text %q folded %d registrations", text, len(g.Delayed))
		}
		s := g.Delayed[0].RememberedStamps[0]
		if s.Obj != id {
			t.Fatalf("Text %q stamped object %d, want %d", text, s.Obj, id)
		}
		return s, o.ZoneGen
	}
	id := func(s state.TargetStamp) state.ObjID { return s.Obj }
	s, live := fold("EndCombat|RS=1:1:3")
	if live != 7 || id(s) != 1 || s.Incarnation != 1 || s.ZoneGen != 3 {
		t.Errorf("three fields: stamp %+v (live ZoneGen %d), want Obj 1 Incarnation 1 ZoneGen 3 against live 7", s, live)
	}
	s, live = fold("EndCombat|RS=1:1")
	if live != 7 || s.Incarnation != 1 || s.ZoneGen != live {
		t.Errorf("legacy two fields: stamp %+v, want the live ZoneGen %d", s, live)
	}
	s, live = fold("EndCombat|RS=1:1:x")
	if live != 7 || s.Incarnation != 1 || s.ZoneGen != live {
		t.Errorf("malformed third field: stamp %+v, want the live ZoneGen %d", s, live)
	}
}
