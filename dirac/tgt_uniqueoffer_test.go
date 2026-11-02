package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestDistinctChainCastOffer: Incremental Growth's three TargetUnique$
// slots need three distinct creatures; with two on the table each slot has
// a candidate but the cast is not offered (tgt_uniqueoffer.go).
func TestDistinctChainCastOffer(t *testing.T) {
	deck := []cardset.Entry{{Name: "Incremental Growth", Count: 1}, {Name: "Hill Giant", Count: 3},
		{Name: "Forest", Count: 36}}
	set := cardsettest.Set(t, "counteradded")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for _, tc := range []struct {
		giants int
		want   bool
	}{{1, false}, {2, false}, {3, true}} {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		for i := 0; i < 5; i++ {
			sc.perm(id["Forest"], 0)
		}
		for i := 0; i < tc.giants; i++ {
			sc.perm(id["Hill Giant"], uint8(i%2))
		}
		ig := sc.hand(id["Incremental Growth"], 0)
		if got := offersCast(sc.s, ig); got != tc.want {
			t.Fatalf("%d creatures: offersCast(Incremental Growth) = %v, want %v", tc.giants, got, tc.want)
		}
	}
}
