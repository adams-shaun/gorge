package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestGraveyardCastPricedAfterItsMove: a spell cast from the graveyard leaves
// it before its cost is determined (CR 601.2a, 601.2f), so Wilt in the Heat's
// "costs {2} less if cards left your graveyard this turn" is met by its own
// cast. The price read while the card still sits in the graveyard (the offer
// and the payment plan made from it) must be the price the cast charges,
// and a copy kept in the hand is not reduced.
func TestGraveyardCastPricedAfterItsMove(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	wilt, ok := reg.Lookup("Wilt in the Heat")
	if !ok {
		t.Fatal("Wilt in the Heat missing from corpus")
	}
	e := corpusEngine(t, reg, []*cards.Card{wilt, wilt}, nil)
	inGrave := moveByName(t, e, 0, "Wilt in the Heat", state.ZGraveyard)
	inHand := moveByName(t, e, 0, "Wilt in the Heat", state.ZHand)
	if inGrave == inHand {
		t.Fatal("setup placed both copies on one object")
	}
	price := func(id state.ObjID) Cost {
		mods := e.costModifiers(0, id, spellScope(""))
		return mods.Apply(e.parseCost(e.G.Obj(id).Face().ManaCost))
	}
	if got := price(inGrave); got.Generic != 0 {
		t.Fatalf("graveyard cast priced %+v, want the {2} reduction applied", got)
	}
	if got := price(inHand); got.Generic != 2 {
		t.Fatalf("hand cast priced %+v, want the printed {2}", got)
	}
}
