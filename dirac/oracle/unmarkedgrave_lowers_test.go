package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A '!' negated supertype word (Unmarked Grave's ChangeType$ Card.!Legendary:
// search your library for a nonlegendary card) in lockstep with gorge: the
// fixture pool (dirac/cardset/pools/fixtures/unmarkedgrave) plays the card in
// a library holding a legendary creature, plain creatures and basics, so the
// search offers every nonlegendary card and never Isamaru (dirac/kgen
// filter.go's '!' supertype arm, the nonLegendary clause; gorge
// matchPredicate's '!' over the Legendary type word).

var unmarkedGraveWatch = map[string]bool{"Unmarked Grave": true}

var unmarkedGraveDeck = []cardset.Entry{
	{Name: "Unmarked Grave", Count: 8}, {Name: "Isamaru, Hound of Konda", Count: 6}, {Name: "Savannah Lions", Count: 4},
	{Name: "Elite Vanguard", Count: 4}, {Name: "Raise Dead", Count: 2}, {Name: "Swamp", Count: 8}, {Name: "Plains", Count: 8},
}

var unmarkedGraveDecks = []fixtureDecks{{unmarkedGraveDeck, unmarkedGraveDeck}}

// dropNotLegendary clears the negated Legendary bit of every table filter
// (the lowering reverted to any card), on a copy.
func dropNotLegendary(t *testing.T) func(*kop.Table) {
	return func(tbl *kop.Table) {
		tbl.Filters = append([]kop.Filter(nil), tbl.Filters...)
		n := 0
		for i := range tbl.Filters {
			if f := &tbl.Filters[i]; f.NotTypes&kop.TLegendary != 0 {
				f.NotTypes &^= kop.TLegendary
				n++
			}
		}
		if n == 0 {
			t.Fatal("precondition: no table filter negates Legendary")
		}
	}
}

func TestUnmarkedGraveLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "unmarkedgrave", unmarkedGraveDecks, unmarkedGraveWatch, 120, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if n := run.casts["Unmarked Grave"]; n < 40 {
		t.Errorf("Unmarked Grave cast %d times", n)
	}
	mut := runFixtureLockstep(t, "unmarkedgrave", unmarkedGraveDecks, unmarkedGraveWatch, 120, false, dropNotLegendary(t))
	mut.requireDivergence(t, "the !Legendary negation reverted")
}
