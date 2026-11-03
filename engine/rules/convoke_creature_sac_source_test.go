package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A creature announced for convoke is spoken for: it is tapped when the cast's
// costs are paid (CR 601.2h), after the mana abilities of 601.2g, so a mana
// source whose cost sacrifices a creature must take another one (CR 601.2g-h,
// 702.51a).

const (
	convokeSacAltar = "Name:Test Pyre Block\nTypes:Artifact\nA:AB$ Mana | Cost$ Sac<1/Creature> | Produced$ B | SpellDescription$ One black.\nOracle:x\n"
)

func TestAltarSacrificesTheOtherCreatureNotTheConvoker(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9701, convokeTapRite)
	toMain1(t, e)
	convoker := onBoardReady(t, e, 0, apaDrone)
	other := onBoardReady(t, e, 0, apaDrone)
	altar := onBoard(t, e, 0, convokeSacAltar)
	idx := -1
	for _, o := range ppPriority(t, e).Options {
		if o.Kind == "cast" && o.Obj == spell {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("the rite is not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, idx)
	choice := -1
	for _, o := range e.Pending().Options {
		if o.Kind == "convoke_generic" && o.Obj == convoker {
			choice = o.Index
		}
	}
	if choice < 0 {
		t.Fatalf("the convoker is not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, choice)
	if d := e.Pending(); d == nil || d.Kind != decision.KChoose {
		t.Fatalf("want the mana window, got %+v", d)
	}
	submitChoices(t, e, activateOption(t, e, altar))
	if d := e.Pending(); d != nil {
		for _, o := range d.Options {
			if o.Kind == "sacrifice" && o.Obj == convoker {
				t.Fatalf("the convoker is offered as the sacrifice: %+v", d.Options)
			}
		}
	}
	for e.Pending() != nil && e.G.Obj(spell).Zone != state.ZStack {
		d := e.Pending()
		if d.Kind != decision.KChoose {
			break
		}
		submitChoices(t, e, 0)
	}
	if e.G.Obj(spell).Zone != state.ZStack {
		t.Fatalf("spell is in %v, want the stack", e.G.Obj(spell).Zone)
	}
	if e.G.Obj(convoker).Zone != state.ZBattlefield || !e.G.Obj(convoker).Tapped {
		t.Fatalf("convoker in %v tapped %v: want it on the battlefield, tapped", e.G.Obj(convoker).Zone, e.G.Obj(convoker).Tapped)
	}
	if e.G.Obj(other).Zone == state.ZBattlefield {
		t.Fatalf("the other creature is still on the battlefield: the altar paid with it")
	}
}
