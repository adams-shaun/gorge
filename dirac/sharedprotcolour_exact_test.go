package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSharedProtColourExact: the SharedKeywordsZone$ PumpAll protection guard
// (kop.W7HasProtColor) asks for the exact "Protection from <colour>" spelling.
// A permanent with only the "Protection from each color" spelling (all five
// colours in one derived keyword) must not answer the black word: gorge's
// SharedKeywordSpellings shares exact spellings, so the lowering would grant
// an extra "Protection from black".
func TestSharedProtColourExact(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	const all = kop.ColW | kop.ColU | kop.ColB | kop.ColR | kop.ColG
	each := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Each Color", Types: kop.TCreature, Power: 1, Toughness: 1, ProtColors: all},
		kop.CardDef{Name: "Test Black", Types: kop.TCreature, Power: 1, Toughness: 1, ProtColors: kop.ColB},
	)
	tbl.Filters = append(tbl.Filters,
		kop.Filter{Types: kop.TCreature, Pred: kop.PredWord7, PredN: kop.W7HasProtColor, PredV: kop.Const(int(kop.ColB))},
	)
	f := kop.FilterID(len(tbl.Filters) - 1)
	sc := newScenTable(t, tbl, Config{})
	ec, bl := sc.perm(each, 0), sc.perm(each+1, 0)
	if sc.s.matchPerm(sc.k, f, sc.slot(ec), 0, NoSlot) {
		t.Fatal(`"Protection from each color" satisfied the "Protection from black" word`)
	}
	if !sc.s.matchPerm(sc.k, f, sc.slot(bl), 0, NoSlot) {
		t.Fatal(`"Protection from black" did not satisfy the black word`)
	}
}
