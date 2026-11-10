package dirac

import (
	"math/bits"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPlaneswalkerTypesCount: ExCount N 1 is the number of distinct
// planeswalker types (CR 205.3j) among the controller's planeswalkers --
// two Ajanis and a Chandra read 2 -- while N 0 still counts the permanents.
// A non-planeswalker's subtype adds nothing; an opponent's walker is not
// matched.
func TestPlaneswalkerTypesCount(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	si := len(tbl.Subtypes)
	tbl.Subtypes = append(tbl.Subtypes, "Ajani", "Chandra", "Gorgon")
	if si+3 > kop.MaxSubtypes {
		t.Fatalf("subtype table too full: %d", si)
	}
	n := CardID(len(tbl.Cards))
	sub := func(i int) kop.SubMask { return kop.SubMask{}.With(si + i) }
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "T Ajani A", Types: kop.TPlaneswalker | kop.TLegendary, Subtypes: sub(0), Loyalty: 3},
		kop.CardDef{Name: "T Ajani B", Types: kop.TPlaneswalker | kop.TLegendary, Subtypes: sub(0), Loyalty: 3},
		kop.CardDef{Name: "T Chandra", Types: kop.TPlaneswalker | kop.TLegendary, Subtypes: sub(1), Loyalty: 3},
		kop.CardDef{Name: "T Gorgon", Types: kop.TCreature, Subtypes: sub(2), Power: 2, Toughness: 4})
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TPlaneswalker, Ctrl: kop.CtrlYou})
	fw := kop.FilterID(len(tbl.Filters) - 1)
	tbl.Filters = append(tbl.Filters, kop.Filter{Ctrl: kop.CtrlYou})
	fall := kop.FilterID(len(tbl.Filters) - 1)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	read := func(x kop.Expr) int {
		sc.k.t.Exprs = append(sc.k.t.Exprs, x)
		return sc.s.expr(sc.k, int16(len(sc.k.t.Exprs)-1), evalCtx{src: NoObj, srcSl: NoSlot, ctrl: 0})
	}
	types := kop.Expr{Kind: kop.ExCount, N: 1, F: fw}
	if got := read(types); got != 0 {
		t.Fatalf("no planeswalkers: %d types, want 0", got)
	}
	sc.perm(n, 0)
	sc.perm(n+1, 0)
	sc.perm(n+2, 0)
	sc.perm(n+2, 1) // the opponent's walker is not the controller's
	sc.perm(n+3, 0) // a creature's subtype is no planeswalker type
	if got := read(kop.Expr{Kind: kop.ExCount, F: fw}); got != 3 {
		t.Fatalf("precondition: %d planeswalkers you control, want 3", got)
	}
	if got := read(types); got != 2 {
		t.Errorf("two Ajanis and a Chandra: %d planeswalker types, want 2", got)
	}
	if got := read(kop.Expr{Kind: kop.ExCount, N: 1, F: fall}); got != 2 {
		t.Errorf("every permanent you control: %d planeswalker types, want 2 (the Gorgon adds none)", got)
	}
}

// TestPlaneswalkerTypesKeepWalkerSubtypes: a matchup whose table counts
// planeswalker types keeps every planeswalker's printed subtypes, which
// nothing else reads, so the count sees them; two Ajanis share one.
func TestPlaneswalkerTypesKeepWalkerSubtypes(t *testing.T) {
	deck := []cardset.Entry{{Name: "Tam, the Possibility", Count: 4}, {Name: "Ajani, Mentor of Heroes", Count: 3},
		{Name: "Ajani, Caller of the Pride", Count: 3}, {Name: "Chandra, Novice Pyromancer", Count: 3},
		{Name: "Forest", Count: 27}}
	tbl, _, _, err := cardsettest.Set(t, "pwtypes").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	hasPWTypes := false
	for _, x := range tbl.Exprs {
		hasPWTypes = hasPWTypes || x.Kind == kop.ExCount && x.N == 1
	}
	if !hasPWTypes {
		t.Fatal("precondition: Tam's table holds no planeswalker-types count")
	}
	var seen kop.SubMask
	walkers := 0
	for _, c := range tbl.Cards {
		if c.Types&kop.TPlaneswalker == 0 {
			continue
		}
		walkers++
		if !c.Subtypes.Any() {
			t.Errorf("planeswalker %+v lost its printed subtypes", c.Loyalty)
		}
		seen = seen.Or(c.Subtypes)
	}
	if n := bits.OnesCount64(seen[0]) + bits.OnesCount64(seen[1]); walkers != 3 || n != 2 {
		t.Errorf("%d walkers with %d planeswalker types, want 3 with 2 (Ajani, Chandra)", walkers, n)
	}
}
