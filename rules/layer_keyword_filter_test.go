package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestWithKeywordPredicateSeesLayer6Grant(t *testing.T) {
	e := layerEngine(t)
	bear := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	obj := e.G.Obj(bear)
	if obj == nil || obj.Zone != state.ZBattlefield {
		t.Fatalf("bear precondition: object=%v zone is not battlefield", obj)
	}
	if obj.Face() == nil || obj.Face().HasKeyword("Menace") {
		t.Fatal("bear must not print Menace")
	}
	sc := e.specCtx(0, 0)
	if e.matchesSpec("Creature.YouCtrl+withMenace", bear, sc) {
		t.Fatal("ungranted bear matched withMenace")
	}
	if !e.matchesSpec("Creature.YouCtrl+withoutMenace", bear, sc) {
		t.Fatal("ungranted bear did not match withoutMenace")
	}

	e.AddContinuous(ContinuousEffect{Source: bear, Timestamp: 1, Layer: LAbilities,
		Affects: "Creature.YouCtrl", Controller: 0, AddKeywords: []string{"Menace"}, UntilEOT: true})
	if !e.HasKeyword(bear, "Menace") {
		t.Fatal("layer-6 Menace grant is not derived")
	}
	sc = e.specCtx(0, 0)
	with := e.matchesSpec("Creature.YouCtrl+withMenace", bear, sc)
	without := e.matchesSpec("Creature.YouCtrl+withoutMenace", bear, sc)
	if !with || without {
		t.Fatalf("derived keyword filter = with %v, without %v; want true, false", with, without)
	}

	e.EndOfTurnCleanup()
	if e.HasKeyword(bear, "Menace") {
		t.Fatal("expired Menace grant remains derived")
	}
	sc = e.specCtx(0, 0)
	with = e.matchesSpec("Creature.YouCtrl+withMenace", bear, sc)
	without = e.matchesSpec("Creature.YouCtrl+withoutMenace", bear, sc)
	if with || !without {
		t.Fatalf("expired keyword filter = with %v, without %v; want false, true", with, without)
	}
}
