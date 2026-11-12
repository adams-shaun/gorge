package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Madame Null, Power Broker's enter trigger costs "life equal to its power",
// and that power is read when the ability resolves -- by then the entering
// creature may already have left the battlefield. A card read off the
// graveyard face has lost every continuous effect: under Glorious Anthem
// Grizzly Bears are 3/3 as they leave and 2/2 dead. The window's X must read
// the last-known battlefield power (CR 603.10, 608.2h), the same read the
// ability's own body binds at resolution; the printed face would undercharge
// the cost by the anthem's one.
func TestTriggerCostXReadsDepartedCardLastKnownPower(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	null, _ := reg.Lookup("Madame Null, Power Broker")
	anthem, _ := reg.Lookup("Glorious Anthem")
	altar, _ := reg.Lookup("Ashnod's Altar")
	bears, _ := reg.Lookup("Grizzly Bears")
	deck := []*cards.Card{null, anthem, altar, bears}
	deck = append(deck, mountainDeck(t, 40-len(deck))...)
	cfg := seatZeroStart(Config{Seed: 208, Names: []string{"null", "opponent"},
		Decks: [][]*cards.Card{deck, mountainDeck(t, 40)}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	for _, name := range []string{"Madame Null, Power Broker", "Glorious Anthem", "Ashnod's Altar"} {
		moveByName(t, e, 0, name, state.ZBattlefield)
	}
	bearsID := searchMoveByName(t, e, "Grizzly Bears", state.ZHand)
	addMana(t, e, 0, "CG")

	// Precondition the payment amount leans on: Madame Null watches another
	// creature enter, and the anthem will be on her controller's battlefield.
	if findBattlefield(t, e, 0, "Glorious Anthem", 0) == 0 {
		t.Fatal("precondition: no Glorious Anthem on the battlefield")
	}
	lifeBefore := e.G.Players[0].Life

	// Cast the Bears and let them resolve: the trigger parks on the stack.
	d := e.Pending()
	castIdx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == bearsID {
			castIdx = o.Index
		}
	}
	if castIdx < 0 {
		t.Fatalf("no cast option for the Bears: %+v", d.Options)
	}
	submitChoices(t, e, castIdx)
	passPriority(t, e)
	passPriority(t, e)
	if !onBattlefield(e, bearsID) {
		t.Fatal("precondition: the Bears never entered")
	}
	if got := e.Power(bearsID); got != 3 {
		t.Fatalf("precondition: Bears power on the battlefield = %d, want 3 (anthem applied)", got)
	}

	// The trigger waits on the stack. Sacrifice the Bears to Ashnod's Altar:
	// the sole creature, so the cost is forced and no sacrifice ask is posed.
	altarID := findBattlefield(t, e, 0, "Ashnod's Altar", 0)
	d = e.Pending()
	altIdx := -1
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Obj == altarID {
			altIdx = o.Index
		}
	}
	if altIdx < 0 {
		t.Fatalf("no activation option for Ashnod's Altar: %+v", d.Options)
	}
	submitChoices(t, e, altIdx)
	// Madame Null herself is also a creature, so the sacrifice cost poses a
	// one-of-two ask; the answer the test needs is the Bears.
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		chooseObjOption(t, e, d, bearsID)
	}
	if o := e.G.Obj(bearsID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Bears zone = %+v, want the graveyard (sacrificed under the trigger)", o)
	}
	passPriority(t, e)
	passPriority(t, e)

	// The window opens: "you may pay life equal to its power". The Bears left
	// the battlefield while the trigger sat there, so the power is their
	// last-known 3, not the graveyard face's 2.
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "trigger_cost_pay" {
		t.Fatalf("precondition: no trigger-cost pay window: %+v", d)
	}
	if len(d.Options) != 2 {
		t.Fatalf("precondition: window options = %+v, want pay and decline", d.Options)
	}
	chooseKindOption(t, e, d, "trigger_cost_pay")

	if got := lifeBefore - e.G.Players[0].Life; got != 3 {
		t.Fatalf("the window paid %d life, want 3 (the Bears' last-known battlefield power)", got)
	}
	replayCheck(t, e, cfg)
}
