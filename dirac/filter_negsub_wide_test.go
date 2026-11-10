package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFilterFifthNegatedSubtype: the matcher reads all four extra negated
// subtypes, so a card whose subtype is the FIFTH negation does not match, while
// one with none of the five does. The synthetic cards and the filter share one
// table, as the table emitter writes them.
func TestFilterFifthNegatedSubtype(t *testing.T) {
	set := cardsettest.Set(t, "selfcond")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	lib, ok := set.Lookup("Forest")
	if !ok {
		t.Fatal("Forest not in the selfcond pool")
	}
	libID := CardID(set.PoolID(lib))

	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	tbl.Filters = append([]kop.Filter(nil), tbl0.Filters...)
	// The pool names only the subtypes its filters read, so name five here: the
	// matcher's read is the index, not the spelling.
	tbl.Subtypes = append([]string(nil), tbl0.Subtypes...)
	for len(tbl.Subtypes) < 5 {
		tbl.Subtypes = append(tbl.Subtypes, fmt.Sprintf("TestSubtype%d", len(tbl.Subtypes)))
	}
	five := []uint8{0, 1, 2, 3, 4}
	// The fifth negation sits in the last extra slot, after NotSub.
	f := kop.Filter{NotSub: five[0] + 1}
	copy(f.NotSubs[:], []uint8{five[1] + 1, five[2] + 1, five[3] + 1, five[4] + 1})
	filled := 0
	for _, n := range f.NotSubs {
		if n != 0 {
			filled++
		}
	}
	if filled != 4 {
		t.Fatalf("the filter holds %d extra negations, want 4", filled)
	}
	tbl.Filters = append(tbl.Filters, f)
	fid := kop.FilterID(len(tbl.Filters) - 1)

	fifthID := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Fifth", Types: kop.TCreature,
		Subtypes: kop.SubMask{}.With(int(five[4]))})
	noneID := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Vanilla", Types: kop.TCreature})

	sc := newScenTableLib(t, &tbl, libID, Config{NoElide: true})
	if sc.s.matchCard(sc.k, fid, fifthID, 0, 0) {
		t.Error("a card with the fifth negated subtype matched")
	}
	if !sc.s.matchCard(sc.k, fid, noneID, 0, 0) {
		t.Error("a card with none of the negated subtypes did not match")
	}
}
