package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

func TestZZDumpSubs(t *testing.T) {
	deck := gyGrantDeck()
	var lists [][]cardset.Entry
	for i := 0; i < 4; i++ {
		es := make([]cardset.Entry, len(deck))
		for j, n := range deck {
			es[j] = cardset.Entry{Name: n, Count: 1}
		}
		lists = append(lists, es)
	}
	set := cardset.Raw(cardsettest.Synth(t, "dr-mbc-aaaa"))
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("four-seat matchup subtypes=%d cards=%d", len(tbl.Subtypes), len(tbl.Cards))
	pool, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("mbc pool subtypes=%d cards=%d", len(pool.Subtypes), len(pool.Cards))
	t.Fatalf("dump")
}
