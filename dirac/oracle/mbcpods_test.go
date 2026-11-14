package oracle

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// mbcSliceCorpus is the committed slice of the mbc pool (draftsim's 50-card
// Mystery Booster Commander draft decks, decks.py's basics for the cards the
// table does not lower) as a corpus, with its pool rows and the mbc table in
// the new-pool mode (kfuzz -allow-unverified: most MBC cards have no evidence
// yet). It skips without the card corpus or the tables.
func mbcSliceCorpus(t *testing.T) (*Corpus, []PoolDeck, cardset.Set) {
	t.Helper()
	base, _ := testCorpus(t)
	set, err := cardset.ByName("mbc")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no mbc table (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	dir, remove, err := ExtractSlice("mbc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(remove)
	pool, err := ReadPool(filepath.Join(dir, "pool.tsv"), "all")
	if err != nil {
		t.Fatal(err)
	}
	c := &Corpus{Reg: base.Reg, DecksDir: filepath.Join(dir, "decks"), decks: map[string][]*cards.Card{}}
	return c, pool, cardset.AllowUnverified(set)
}

// TestMBCPodTables: the mbc pool's table holds more subtypes than one game's
// table can (kop.MaxSubtypes), and every one of the first 200 four-seat pods of
// its drafted decks (kfuzz -kernel mbc -seats 4) still builds its own table
// within it: a game's table keeps only the subtypes its own cards print or name
// (cardset matchup). A pod the kernel refuses for a card interaction (or an
// older guard: layer, Room name read) is counted;
// a SubtypeRefusal, or any other error, fails.
func TestMBCPodTables(t *testing.T) {
	c, pool, set := mbcSliceCorpus(t)
	if n := len(set.Subtypes()); n <= kop.MaxSubtypes {
		t.Fatalf("precondition: the mbc pool holds %d subtypes, no more than one table's %d", n, kop.MaxSubtypes)
	}
	built, refused, most := 0, 0, 0
	reasons := map[string]int{}
	for i := range 200 {
		p := PodAt(pool, i, SliceSeed, 4)
		es := make([][]cardset.Entry, 0, 4)
		for _, n := range p.Decks() {
			f, err := c.deckFile(n)
			if err != nil {
				t.Fatal(err)
			}
			var d []cardset.Entry
			for _, e := range f.Cards {
				d = append(d, cardset.Entry{Name: e.Name, Count: e.Count})
			}
			es = append(es, d)
		}
		tbl, _, err := set.MatchupN(es)
		if why := stdMetaPairRefusal(err); why != "" && why != "interaction subtypes" {
			refused++
			reasons[why]++
			continue
		}
		if err != nil {
			t.Fatalf("pod %d %v: %v", i, p.Decks(), err)
		}
		built++
		most = max(most, len(tbl.Subtypes))
	}
	t.Logf("pool %d subtypes; 200 pods: %d built (most subtypes %d), %d refused as interactions %v", len(set.Subtypes()), built, most, refused, reasons)
	if built == 0 {
		t.Fatal("no pod built a table")
	}
}
