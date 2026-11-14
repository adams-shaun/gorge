package oracle

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRealKernelMapsGorgeNames: a table naming a card by the pool decklist's
// spelling (an unaccented name, a full "A // B" Room name) maps to the card
// gorge prints.
func TestRealKernelMapsGorgeNames(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	dain, ok := reg.Lookup("Dáin's Company")
	if !ok {
		t.Fatal("Dáin's Company not in corpus")
	}
	annex, ok := reg.Lookup("Unholy Annex")
	if !ok {
		t.Fatal("Unholy Annex not in corpus")
	}
	tbl := &kop.Table{Name: "names", Cards: []kop.CardDef{{Name: "Dain's Company"}, {Name: "Unholy Annex // Ritual Chamber"}}}
	m, err := NewMatchup(reg, [2]string{"a", "b"}, [2][]*cards.Card{{dain, annex}, {dain, annex}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	side, err := RealKernel(tbl, nil, Config{})(m, 1)
	if err != nil {
		t.Fatalf("RealKernel: %v", err)
	}
	r := side.(*realSide)
	for id, printed := range []string{cardName(dain), cardName(annex)} {
		if tbl.Cards[id].Name == printed {
			t.Fatalf("precondition: table name %q equals the printed name", printed)
		}
		if r.gname[id] != printed {
			t.Errorf("gname[%d] = %q, want %q", id, r.gname[id], printed)
		}
		j := sort.SearchStrings(m.Table, printed)
		if j >= len(m.Table) || m.Table[j] != printed {
			t.Fatalf("%q not in the matchup table %v", printed, m.Table)
		}
		if r.tidx[id] != CardIndex(j) {
			t.Errorf("tidx[%d] = %d, want %d", id, r.tidx[id], j)
		}
	}
}
