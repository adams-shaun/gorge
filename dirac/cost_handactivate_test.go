package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCyclingFromHand: a hand ability whose cost discards the card
// (cycling's expansion, CR 702.29a: "{1}, Discard this card: Draw a card")
// is offered from the hand, the card is discarded as the cost, and the
// ability draws on resolution.
func TestCyclingFromHand(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	plains := da[0]
	cyc := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Cycler", Types: kop.TSorcery, Cost: kop.ManaCost{Generic: 5},
		Abilities: []kop.Ability{{Kind: kop.AbActivated, ActZone: kop.ZoneHand,
			Cost: kop.Cost{Mana: kop.ManaCost{Generic: 1}, Flags: kop.CostDiscardSelf},
			Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)}}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.lands(0, plains, 1)
	c := sc.hand(cyc, 0)
	hand0 := sc.s.pl[0].handN
	sc.do(t, AkActivate, TCode(c), 0)
	sc.s.Step() // the build completes: costs paid, the ability on the stack
	if z := sc.s.objs[c].zone; z != kop.ZoneGraveyard || sc.s.sp != 1 {
		t.Fatalf("the cycled card is in zone %d (stack %d), want the graveyard", z, sc.s.sp)
	}
	sc.s.Apply(MakeAction(AkPass, 0xff, 0xff, 0))
	sc.do(t, AkPass, scAny, scAny)
	if sc.s.sp != 0 || sc.s.pl[0].handN != hand0 {
		t.Fatalf("stack %d, hand %d (want %d: the cycler out, one card drawn)", sc.s.sp, sc.s.pl[0].handN, hand0)
	}
}
