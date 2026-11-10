package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestFoldDelayedRegisterCaptureOnly pins the "|CO" Text spelling of a
// capture-only registration (state.DelayedTrigger.CaptureOnly): it rides
// immediately before |RN=, folds to the flag, and leaves the neighbouring
// slots intact. The same Text without it (an old log) folds to false.
func TestFoldDelayedRegisterCaptureOnly(t *testing.T) {
	fold := func(text string) state.DelayedTrigger {
		t.Helper()
		g := state.NewGame([]string{"Ann", "Bob"})
		card, _ := cards.ParseBytes("c.txt", []byte("Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"))
		id := g.AddObject(card, 0).ID
		g.SetZone(state.ZBattlefield, 0, []state.ObjID{id})
		g.Obj(id).Zone = state.ZBattlefield
		Apply(g, Event{Kind: DelayedRegister, Obj: id, Player: 0, Step: state.StepEndCombat,
			Counter: "Body", IDs: []state.ObjID{id}, Text: text})
		if len(g.Delayed) != 1 {
			t.Fatalf("Text %q folded %d registrations, want 1", text, len(g.Delayed))
		}
		return g.Delayed[0]
	}
	with := fold("EndCombat|VP=Player|CO|RN=3|RS=1:1")
	if !with.CaptureOnly || with.RememberedAmount != 3 {
		t.Errorf("with |CO: CaptureOnly=%v RememberedAmount=%d, want true and 3", with.CaptureOnly, with.RememberedAmount)
	}
	without := fold("EndCombat|VP=Player|RN=3|RS=1:1")
	if without.CaptureOnly || without.RememberedAmount != 3 {
		t.Errorf("without |CO: CaptureOnly=%v RememberedAmount=%d, want false and 3", without.CaptureOnly, without.RememberedAmount)
	}
	bare := fold("EndCombat|VP=Player|CO|RS=1:1")
	if !bare.CaptureOnly || bare.RememberedAmount != 0 {
		t.Errorf("|CO with no |RN=: CaptureOnly=%v RememberedAmount=%d, want true and 0", bare.CaptureOnly, bare.RememberedAmount)
	}
	if with.Phase != without.Phase || with.Phase != state.StepEndCombat {
		t.Errorf("phase differs: %v vs %v", with.Phase, without.Phase)
	}
}
