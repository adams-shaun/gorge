package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSplitCardInHandSharesEitherHalfColour: a split card off the stack has
// both halves' colours (CR 709.4b), so a "discard a card that shares a colour
// with the spell" part counts one in hand whose second half alone shares it.
func TestSplitCardInHandSharesEitherHalfColour(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, da, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	front := CardID(len(tbl.Cards))
	back, white := front+1, front+2
	faces := []CardID{front, back}
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Left", Types: kop.TSorcery, Colors: 1 << kop.G, FaceMode: kop.FaceSplit, Faces: faces},
		kop.CardDef{Name: "Test Right", Types: kop.TSorcery, Colors: 1 << kop.W, FaceMode: kop.FaceSplit, Face: 1, Faces: faces},
		kop.CardDef{Name: "Test White", Types: kop.TSorcery, Colors: 1 << kop.W})
	sc := newScenTableLib(t, tbl, da[0], Config{NoElide: true})
	if !sc.k.gen.anySplit {
		t.Fatal("precondition: the table has no split card")
	}
	split := sc.hand(front, 0)
	cast := sc.hand(white, 0)
	if tbl.Cards[front].Colors&tbl.Cards[white].Colors != 0 {
		t.Fatal("precondition: the front half already shares the cast card's colour")
	}
	if !sc.s.handSharesColor(sc.k, split, cast) {
		t.Fatal("the split card in hand does not share white through its second half")
	}
}
