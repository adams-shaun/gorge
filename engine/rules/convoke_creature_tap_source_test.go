package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A creature announced for convoke is spoken for: the mana window must not let
// a tap-a-creature mana source (a drum) spend the same creature as its own
// cost, and a creature that source taps must not also have been the
// convoker (CR 601.2g-h, 702.51a).

const (
	convokeTapRite = "Name:Test Rally\nManaCost:2\nTypes:Sorcery\nK:Convoke\nA:SP$ GainLife | LifeAmount$ 1 | SpellDescription$ Life.\nOracle:x\n"
	convokeTapDrum = "Name:Test Beat\nTypes:Artifact\nA:AB$ Mana | Cost$ T tapXType<1/Creature> | Produced$ Any | SpellDescription$ One mana.\nOracle:x\n"
)

// convokeTapStart casts the rite, convokes with `convoker` and returns the
// pending decision that follows.
func convokeTapStart(t *testing.T, extra bool) (e *Engine, drum, convoker, other state.ObjID, spell state.ObjID) {
	t.Helper()
	e, _, spell = newFixtureDeck(t, 9700, convokeTapRite)
	toMain1(t, e)
	convoker = onBoardReady(t, e, 0, apaDrone)
	if extra {
		other = onBoardReady(t, e, 0, apaDrone)
	}
	drum = onBoard(t, e, 0, convokeTapDrum)
	e.G.Obj(drum).SummonSick = false
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
	ask := e.Pending()
	if ask == nil || ask.Kind != decision.KChoose || ask.Source != spell {
		t.Fatalf("pending %+v, want the convoke ask", ask)
	}
	choice := -1
	for _, o := range ask.Options {
		if o.Kind == "convoke_generic" && o.Obj == convoker {
			choice = o.Index
		}
	}
	if choice < 0 {
		t.Fatalf("the convoker is not offered: %+v", ask.Options)
	}
	submitChoices(t, e, choice)
	return
}

// With a second creature the drum's cost is forced onto it, never the convoker.
func TestDrumTapsTheOtherCreatureNotTheConvoker(t *testing.T) {
	t.Parallel()
	e, drum, convoker, other, spell := convokeTapStart(t, true)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("want the mana window, got %+v", d)
	}
	submitChoices(t, e, activateOption(t, e, drum))
	if d := e.Pending(); d != nil {
		for _, o := range d.Options {
			if o.Kind == "tapcost" {
				t.Fatalf("a tap election was posed over %+v: the convoker must not be a candidate", d.Options)
			}
		}
	}
	answerManaChoose(t, e, "Add G")
	submitChoices(t, e, 0)
	if e.G.Obj(spell).Zone != state.ZStack {
		t.Fatalf("spell is in %v, want the stack", e.G.Obj(spell).Zone)
	}
	if !e.G.Obj(convoker).Tapped || !e.G.Obj(other).Tapped || !e.G.Obj(drum).Tapped {
		t.Fatalf("convoker %v, other %v, drum %v tapped: want all three", e.G.Obj(convoker).Tapped, e.G.Obj(other).Tapped, e.G.Obj(drum).Tapped)
	}
}
