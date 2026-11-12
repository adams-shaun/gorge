package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Teferi, Time Raveler's static: each opponent can cast spells only any time
// they could cast a sorcery -- not never (the unread OnlySorcerySpeed$ made
// it a blanket CantBeCast), and not at instant speed.
func TestTeferiTimeRaverOpponentCastsAtSorcerySpeedOnly(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Teferi, Time Raveler")},
		[]*cards.Card{lookup(t, reg, "Lightning Bolt"), lookup(t, reg, "Shock")})
	bolt := moveByName(t, e, 1, "Lightning Bolt", state.ZHand)
	shock := moveByName(t, e, 1, "Shock", state.ZHand)
	// Seat 1's own main phase, empty stack: a sorcery-speed window.
	driveToStep(t, e, 2, 1, state.StepMain1)
	addMana(t, e, 1, "RR")
	if d := e.Pending(); d == nil || d.Player != 1 || len(e.G.Stack) != 0 {
		t.Fatalf("precondition: seat 1 should hold priority in its main phase with an empty stack: %+v", d)
	}
	if !castOfferedFor(e, bolt) {
		t.Fatalf("precondition: Bolt is not offered before Teferi is in play: %+v", e.Pending().Options)
	}
	moveByName(t, e, 0, "Teferi, Time Raveler", state.ZBattlefield)
	e.pending = nil
	e.priorityRound()
	if !castOfferedFor(e, bolt) {
		t.Fatalf("Teferi's static withheld an opponent's cast at sorcery timing: %+v", e.Pending().Options)
	}
	castNamed(t, e, "Lightning Bolt")
	answerHybridPayment(t, e)
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: Bolt is not on the stack (%d)", len(e.G.Stack))
	}
	if d := e.Pending(); d == nil || d.Player != 1 {
		t.Fatalf("precondition: seat 1 should hold priority over its own Bolt: %+v", d)
	}
	if castOfferedFor(e, shock) {
		t.Fatalf("an opponent was offered an instant with a spell on the stack under Teferi: %+v", e.Pending().Options)
	}
	replayCheck(t, e, cfg)
}

// Teferi's +1: until your next turn, you may cast sorcery spells as though
// they had flash -- through the end of the opponent's turn, gone when your
// next turn begins.
func TestTeferiTimeRaverPlusOneGrantsSorceryFlashUntilYourNextTurn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Teferi, Time Raveler"),
		lookup(t, reg, "Lightning Bolt"), lookup(t, reg, "Divination"), lookup(t, reg, "Grizzly Bears")}, nil)
	moveByName(t, e, 0, "Teferi, Time Raveler", state.ZHand)
	moveByName(t, e, 0, "Lightning Bolt", state.ZHand)
	div := moveByName(t, e, 0, "Divination", state.ZHand)
	bears := moveByName(t, e, 0, "Grizzly Bears", state.ZHand)
	addMana(t, e, 0, "WUU")
	castNamed(t, e, "Teferi, Time Raveler")
	passUntilStackEmpty(t, e, 40)
	var teferi state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o.Face() != nil && o.Face().Name == "Teferi, Time Raveler" {
			teferi = id
		}
	}
	if teferi == 0 || e.G.Obj(teferi).Counter("LOYALTY") != 4 {
		t.Fatalf("precondition: Teferi is not in play with 4 loyalty (id %d)", teferi)
	}
	opt, ok := findAbilityOptionByLabel(e, teferi, "Until your next turn, you may cast sorcery spells as though they had flash.")
	if !ok {
		t.Fatalf("Teferi's +1 is not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 40)
	boltOnStack(t, e)
	addMana(t, e, 0, "UUUGG")
	if !castOfferedFor(e, div) {
		t.Fatalf("Teferi's +1 did not grant flash to a sorcery: %+v", e.Pending().Options)
	}
	if castOfferedFor(e, bears) {
		t.Fatal("Teferi's +1 (ValidCard$ Sorcery) granted flash to a creature")
	}
	// Through the opponent's turn ...
	driveToStep(t, e, 2, 1, state.StepMain1)
	if !e.castWithFlash(0, div) {
		t.Fatal("the grant ended before the controller's next turn")
	}
	// ... and gone once the controller's next turn begins.
	driveToStep(t, e, 3, 0, state.StepMain1)
	if e.castWithFlash(0, div) {
		t.Fatal("the grant outlived the controller's next turn beginning")
	}
	replayCheck(t, e, cfg)
}
