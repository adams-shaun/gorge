package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

const undauntedSrc = "Name:Undaunted Test\nManaCost:6 G\nTypes:Sorcery\nK:Undaunted\nOracle:x\n"
const plainCostSrc = "Name:Plain Cost Test\nManaCost:6 G\nTypes:Sorcery\nOracle:x\n"

func TestUndauntedReducesGenericCost(t *testing.T) {
	e, cfg, card := newFixtureDeck(t, 1705, undauntedSrc, plainCostSrc)
	o := e.G.Obj(card)
	if o == nil || o.Zone != state.ZHand || o.Face() == nil || !o.Face().HasKeyword("Undaunted") {
		t.Fatalf("precondition: Undaunted spell not in hand with keyword: %+v", o)
	}
	if got := reduceOf(t, e, 0, card); got != 1 {
		t.Fatalf("two-player opponent reduction = %d, want 1", got)
	}
	addMana(t, e, 0, "CCCCC G")
	if opt := castByName(t, e, 0, "Undaunted Test"); opt == nil {
		t.Fatal("spell should cast for {5}{G} with one opponent")
	}
	plain := moveByName(t, e, 0, "Plain Cost Test", state.ZHand)
	if got := reduceOf(t, e, 0, plain); got != 0 {
		t.Fatalf("plain spell unexpectedly reduced: %d", got)
	}
	replayCheck(t, e, cfg)
}
