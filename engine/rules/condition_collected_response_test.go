package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestConditionCollectedSurvivesResponse pins that the Collected window is the
// resolving spell's OWN cast: a response pushed onto the stack after the
// spell's evidence was collected (and resolving before it) must not hide the
// exiled card from CostMovesInWindow.
func TestConditionCollectedSurvivesResponse(t *testing.T) {
	t.Parallel()
	necropsySrc := alltargetedCorpusText(t, "u/urgent_necropsy.txt")
	artifactSrc := "Name:Gold Myr\nManaCost:2\nTypes:Artifact Creature Myr\nPT:1/1\nOracle:x\n"
	grave3Src := "Name:Big Bones\nManaCost:3\nTypes:Artifact\nOracle:x\n"
	zapSrc := "Name:Little Zap\nManaCost:R\nTypes:Instant\nA:SP$ GainLife | LifeAmount$ 1 | SpellDescription$ You gain 1 life.\nOracle:x\n"

	e, _, necro := newFixtureDeck(t, 76, necropsySrc, artifactSrc, grave3Src, zapSrc)
	artifact := putCreature(t, e, 0, artifactSrc)
	mv3 := moveSeeded(t, e, 0, grave3Src, state.ZGraveyard)
	zap := moveSeeded(t, e, 0, zapSrc, state.ZHand)
	addMana(t, e, 0, "BBBGR")
	e.Advance()
	pick := func(d *decision.Decision, obj state.ObjID) int {
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Obj == obj {
				return o.Index
			}
		}
		t.Fatalf("precondition: cast of %d not offered: %+v", obj, d.Options)
		return -1
	}
	submitChoices(t, e, pick(e.Pending(), necro))
	d := e.Pending()
	tgt := -1
	for _, o := range d.Options {
		if o.Obj == artifact {
			tgt = o.Index
		}
	}
	if d.Kind != decision.KTarget || tgt < 0 {
		t.Fatalf("precondition: artifact target ask = %+v", d)
	}
	submitChoices(t, e, tgt)
	submitChoices(t, e) // creature chain sub: elect zero
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "evidence" {
		t.Fatalf("precondition: evidence ask = %+v", d)
	}
	submitChoices(t, e, d.Options[0].Index)

	// Respond to our own spell: the instant is pushed ABOVE the Necropsy.
	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("precondition: priority after the cast = %+v", d)
	}
	submitChoices(t, e, pick(d, zap))
	if len(e.G.Stack) != 2 || e.G.Stack[0] != necro {
		t.Fatalf("precondition: stack = %v, want Necropsy then the response above it", e.G.Stack)
	}
	pushes := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.PutOnStack && ev.Obj == zap {
			pushes++
		}
	}
	if pushes != 1 {
		t.Fatalf("precondition: the response's PutOnStack is not in the log")
	}

	check := func(when string) {
		t.Helper()
		got := e.CostMovesInWindow(necro, events.CostMoveEvidence)
		if len(got) != 1 || got[0] != mv3 {
			t.Fatalf("%s: CostMovesInWindow(CostMoveEvidence) = %v, want [%d]", when, got, mv3)
		}
	}
	check("response on the stack")
	// Resolve the response; the Necropsy is still on the stack.
	castFirst(t, e, "pass")
	castFirst(t, e, "pass")
	if so := e.G.Obj(necro); so == nil || so.Zone != state.ZStack {
		t.Skipf("response resolution consumed the Necropsy too early: %+v", so)
	}
	check("response resolved")
}
