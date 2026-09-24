package events

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// TestClonePermanentGainThisTriggerAppendsOnlyTheTrigger pins the fold level
// of the DB$/SVar-under-trigger GainThisAbility$ form (ticket
// agent-20260923T090459Z-a4d7eb3b): with Counter "gain-this-trigger", the fold
// appends exactly the one indexed trigger to the copied face's Triggers, leaves
// its Abilities untouched, merges the become face's SVars and latches
// CopyGainThisAbility. It is the trigger counterpart of
// TestClonePermanentGainThisAbilityCopiesOnlyNamedAbility.
func TestClonePermanentGainThisTriggerAppendsOnlyTheTrigger(t *testing.T) {
	g := state.NewGame([]string{"Ann", "Bob"})
	target := sailorCard() // one printed ability, no triggers
	// The become object carries one printed ability AND one trigger; the
	// trigger's Effect is the DB body the emblem of a real carrier points at.
	become, _ := cards.ParseBytes("become.txt", []byte("Name:Cryptoplasm Shape\nManaCost:1 U\nTypes:Creature Shapeshifter\nPT:2/2\n"+
		"A:AB$ Draw | Cost$ 3 U | NumCards$ 1 | Defined$ You\n"+
		"T:Mode$ Phase | Phase$ Upkeep | Execute$ CopyBody\n"+
		"SVar:CopyBody:DB$ Clone | GainThisAbility$ True\nOracle:x\n"))
	become.Link()
	if len(target.Faces[0].Abilities) != 1 || len(target.Faces[0].Triggers) != 0 {
		t.Fatalf("precondition: the copied source must have one ability and no triggers, got %d/%d",
			len(target.Faces[0].Abilities), len(target.Faces[0].Triggers))
	}
	if len(become.Faces[0].Triggers) != 1 || become.Faces[0].Triggers[0].Effect == nil {
		t.Fatalf("precondition: the become object must have exactly one compiled trigger, got %+v", become.Faces[0].Triggers)
	}
	if len(become.Faces[0].Abilities) != 1 {
		t.Fatalf("precondition: the become object must have exactly one printed ability, got %d", len(become.Faces[0].Abilities))
	}
	sourceID := g.AddObject(target, 0).ID
	becomeID := g.AddObject(become, 0).ID
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{sourceID, becomeID})
	g.Obj(sourceID).Zone = state.ZBattlefield
	g.Obj(becomeID).Zone = state.ZBattlefield

	Apply(g, Event{Kind: ClonePermanent, Obj: becomeID, IDs: []state.ObjID{sourceID}, Counter: "gain-this-trigger", Amount: 1})
	face := g.Obj(becomeID).Face()
	if len(face.Triggers) != 1 {
		t.Fatalf("gained trigger count = %d, want exactly 1; face: %+v", len(face.Triggers), face)
	}
	if face.Triggers[0].Effect != become.Faces[0].Triggers[0].Effect {
		t.Fatalf("gained trigger Effect = %p, want the indexed become trigger's Effect %p",
			face.Triggers[0].Effect, become.Faces[0].Triggers[0].Effect)
	}
	// Abilities must be EXACTLY the copied source's one, not the become
	// object's own Draw ability too (the whole point of the index form).
	if len(face.Abilities) != 1 {
		t.Fatalf("copied ability count = %d, want exactly the source's 1 (the trigger form must not append abilities); abilities: %+v",
			len(face.Abilities), face.Abilities)
	}
	if !g.Obj(becomeID).CopyGainThisAbility {
		t.Fatal("CopyGainThisAbility must latch for the trigger form")
	}
	if face.SVars["CopyBody"] == "" {
		t.Fatal("the become face's SVar table must merge onto the copy for the gained trigger to resolve")
	}
}
