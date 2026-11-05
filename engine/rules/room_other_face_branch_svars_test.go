package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A Room cast as one half and later unlocked as the other resolves the other
// half's ordinary triggers (here an end-step Branch) against that half's own
// script table (CR 309.6, 113.7a): the Branch's arms are SVar names the cast
// half never defines, so the resolution must bind the unlocked half's table,
// not the shown face's.
func TestRoomOtherFaceTriggerResolvesWithItsOwnSVars(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	room := card(t, "Name:Front Half\nManaCost:0\nTypes:Enchantment Room\nOracle:x\n"+
		"ALTERNATE\nName:Back Half\nManaCost:0\nTypes:Enchantment Room\n"+
		"T:Mode$ Phase | Phase$ End of Turn | Execute$ TrigEndStep | TriggerDescription$ x\n"+
		"SVar:TrigEndStep:DB$ Branch | BranchConditionSVar$ DemonCount | BranchConditionSVarCompare$ GT0 | TrueSubAbility$ DBArmGain | FalseSubAbility$ DBArmLose\n"+
		"SVar:DemonCount:Count$Valid Creature.YouCtrl\n"+
		"SVar:DBArmGain:DB$ GainLife | LifeAmount$ 2 | Defined$ You\n"+
		"SVar:DBArmLose:DB$ LoseLife | LifeAmount$ 2 | Defined$ You\n"+
		"Oracle:y\nAlternateMode:Split\n")
	filler := card(t, "Name:Filler Card\nManaCost:0\nTypes:Sorcery\nOracle:z\n")
	e := corpusEngine(t, reg, []*cards.Card{room, filler, filler, filler}, nil)
	id := moveByName(t, e, 0, "Front Half", state.ZBattlefield)
	designateRoomCastFace(e, id)
	o := e.G.Obj(id)
	if o == nil || o.FaceIdx != 0 || o.RoomOtherDoorUnlocked() {
		t.Fatalf("precondition: Room is not live as its front half with the back door locked: %+v", o)
	}
	e.emit(events.Event{Kind: events.DoorUnlock, Obj: id})
	o = e.G.Obj(id)
	if !o.RoomOtherDoorUnlocked() {
		t.Fatal("precondition: the back door was not unlocked")
	}
	e.priorityRound()
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("precondition: stray pending triggers after the unlock: %d", len(e.pendingTriggers))
	}
	before := e.G.Players[0].Life
	e.G.Active = 0
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepEnd})
	e.priorityRound()
	if len(e.G.Stack) != 1 {
		t.Fatalf("the unlocked half's end-step trigger is not on the stack: %d items", len(e.G.Stack))
	}
	resolveStack(t, e)
	if got := e.G.Players[0].Life; before-got != 2 {
		t.Fatalf("the unlocked half's Branch lost %d life, want 2", before-got)
	}
}
