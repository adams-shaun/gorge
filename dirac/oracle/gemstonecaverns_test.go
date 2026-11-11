package oracle

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestGemstoneCavernsDeferredManaLockstep is the Gemstone Caverns half of the
// Condition-split lockstep (riveroftears_test.go has River of Tears'). Its mana
// ability lowers (dirac/kgen lower_condsplit.go: {C}, or one mana of any colour
// with a luck counter on it; a lucky source asks a colour in the prefix search
// and the window), but the card as a whole stays refused: its opening-hand
// effect ("MayEffectFromOpeningHand ... !PlayFirst": a luck counter and a
// mandatory hand exile, ticket agent-20261111T100500Z-fe5f70d2) is not lowered,
// so no corpus game can hold a lucky Gemstone. No facility poses a counter on a
// permanent before a lockstep game, so there is no real matchup to play yet.
//
// While kgen refuses the card the test skips. The moment kgen lowers it the test
// FAILS: the 2-deck matchup (an unlucky and a lucky Gemstone paying a {C} cast
// and a coloured one, luck counters by the opening-hand route, on both surfaces)
// is then to be written, as TestRiverOfTearsDeferredManaLockstep is.
func TestGemstoneCavernsDeferredManaLockstep(t *testing.T) {
	set, err := cardset.RawByName("gemstonecaverns")
	if err != nil {
		t.Fatal(err)
	}
	deck := []cardset.Entry{{Name: "Gemstone Caverns", Count: 4}, {Name: "Grizzly Bears", Count: 8}, {Name: "Mountain", Count: 28}}
	_, _, _, err = set.Matchup(deck, deck)
	var ref *cardset.RefusedError
	switch {
	case errors.As(err, &ref) && ref.Card == "Gemstone Caverns":
		t.Skip("Gemstone Caverns still refused by kgen (opening-hand node): " + ref.Reason)
	case err != nil:
		t.Fatal(err)
	}
	t.Fatal("kgen now lowers Gemstone Caverns: write the 2-deck lockstep matchup (unlucky and lucky Gemstone payments, luck counters by the opening-hand route, both surfaces) in this test")
}
