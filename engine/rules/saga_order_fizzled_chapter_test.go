package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Two Sagas chapter at once, so their abilities reach the stack through the
// order answer. One is a final chapter with no legal target, removed as it is
// placed (CR 603.3d). The answer's drain ends in a priority decision, and the
// player must not receive it with that Saga still due its sacrifice (CR 117.5,
// 714.4). The other Saga still has a chapter ability to resolve.
func TestOrderedDrainSacrificesASagaWhoseFinalChapterHadNoTarget(t *testing.T) {
	t.Parallel()
	const (
		stranded = "Name:Stranded Saga\nTypes:Enchantment Saga\nK:Chapter:1:Stranded\n" +
			"SVar:Stranded:DB$ Pump | ValidTgts$ Creature.YouCtrl | NumAtt$ 1\nOracle:x\n"
		steady = "Name:Steady Saga\nTypes:Enchantment Saga\nK:Chapter:2:Steady,Steady\n" +
			"SVar:Steady:DB$ GainLife | LifeAmount$ 1\nOracle:x\n"
	)
	for _, order := range [][]int{{0, 1}, {1, 0}} {
		e := handEngine(t)
		toMain1(t, e)
		a := onBoard(t, e, 0, stranded)
		b := onBoard(t, e, 0, steady)
		sagaAddLore(e, a, 1)
		sagaAddLore(e, b, 1)
		e.priorityRound()
		if d := e.Pending(); d == nil || d.Kind != decision.KTriggerOrder {
			t.Fatalf("pending = %+v, want the order ask over both chapters", d)
		}
		submitChoices(t, e, order...)

		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("order %v: pending = %+v, want priority", order, d)
		}
		if z := e.G.Obj(a).Zone; z != state.ZGraveyard {
			t.Fatalf("order %v: priority posed with the Saga whose final chapter left the stack in %s, want the graveyard", order, z)
		}
		if z := e.G.Obj(b).Zone; z != state.ZBattlefield {
			t.Fatalf("order %v: the other Saga is in %s, want the battlefield", order, z)
		}
		if len(e.G.Stack) != 1 || e.G.Obj(e.G.Stack[0]).Source != b {
			t.Fatalf("order %v: stack %v, want only the other Saga's chapter", order, e.G.Stack)
		}
	}
}

// A lone final chapter with no legal target goes straight from the step's own
// priority round, with no answer in between, and its Saga is still sacrificed
// before anyone receives priority.
func TestDirectDrainSacrificesASagaWhoseFinalChapterHadNoTarget(t *testing.T) {
	t.Parallel()
	const stranded = "Name:Stranded Saga\nTypes:Enchantment Saga\nK:Chapter:1:Stranded\n" +
		"SVar:Stranded:DB$ Pump | ValidTgts$ Creature.YouCtrl | NumAtt$ 1\nOracle:x\n"
	e := handEngine(t)
	toMain1(t, e)
	a := onBoard(t, e, 0, stranded)
	sagaAddLore(e, a, 1)
	e.priorityRound()
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("pending = %+v, want priority", d)
	}
	if z := e.G.Obj(a).Zone; z != state.ZGraveyard {
		t.Fatalf("priority posed with the Saga in %s, want the graveyard", z)
	}
}
