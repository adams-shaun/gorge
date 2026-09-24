package events

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

func TestClonePermanentGainThisAbilityCopiesOnlyNamedAbility(t *testing.T) {
	g := state.NewGame([]string{"Ann", "Bob"})
	target := sailorCard()
	become, _ := cards.ParseBytes("become.txt", []byte("Name:Two Abilities\nManaCost:1 U\nTypes:Creature Wizard\nPT:2/2\nA:AB$ Draw | Cost$ 3 U | NumCards$ 1 | Defined$ You\nA:AB$ GainLife | Cost$ 1 | LifeAmount$ 1 | Defined$ You\nOracle:x\n"))
	become.Link()
	sourceID := g.AddObject(target, 0).ID
	becomeID := g.AddObject(become, 0).ID
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{sourceID, becomeID})
	g.Obj(sourceID).Zone = state.ZBattlefield
	g.Obj(becomeID).Zone = state.ZBattlefield
	if g.Obj(sourceID).Zone != state.ZBattlefield || g.Obj(becomeID).Zone != state.ZBattlefield {
		t.Fatal("precondition: both copy operands must be on the battlefield")
	}
	if become.Faces[0].Abilities[0].API == become.Faces[0].Abilities[1].API {
		t.Fatal("precondition: become object's two abilities must be distinct")
	}

	Apply(g, Event{Kind: ClonePermanent, Obj: becomeID, IDs: []state.ObjID{sourceID}, Counter: "gain-this-ability", Amount: 1})
	face := g.Obj(becomeID).Face()
	if len(face.Abilities) != 2 {
		t.Fatalf("copied ability count = %d, want target ability plus exactly one gained ability", len(face.Abilities))
	}
	if face.Abilities[1].API != "Draw" {
		t.Fatalf("gained ability = %q, want Draw (the selected ability); face abilities: %+v", face.Abilities[1].API, face.Abilities)
	}
}
