package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRealKernelMapsStdMetaReportedNames pins the five standard-meta spellings
// kfuzz reported as "not in table": the split card Cease // Desist, the Rooms
// Roaring Furnace // Steaming Sauna and Unholy Annex // Ritual Chamber, and the
// unaccented decklist spellings of Dáin's Company and Thrór's Map. Fixed by
// fd0d11f1af (canon.GorgeName in RealKernel's rev map).
func TestRealKernelMapsStdMetaReportedNames(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	pool := stdMetaCards(t)
	reported := []struct{ table, gorge string }{
		{"Cease // Desist", "Cease"},
		{"Roaring Furnace // Steaming Sauna", "Roaring Furnace"},
		{"Unholy Annex // Ritual Chamber", "Unholy Annex"},
		{"Dain's Company", "Dáin's Company"},
		{"Thror's Map", "Thrór's Map"},
	}
	tbl := &kop.Table{Name: "standard-meta"}
	var deck []*cards.Card
	for _, r := range reported {
		if !pool[r.table] {
			t.Fatalf("precondition: %q is not in standard-meta's committed cards.txt", r.table)
		}
		if r.table == r.gorge {
			t.Fatalf("precondition: table spelling %q equals the printed name", r.table)
		}
		c, ok := reg.Lookup(r.gorge)
		if !ok {
			t.Fatalf("precondition: corpus misses %q", r.gorge)
		}
		if got := cardName(c); got != r.gorge {
			t.Fatalf("precondition: corpus Lookup(%q) prints %q", r.gorge, got)
		}
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: r.table})
		deck = append(deck, c)
	}
	m, err := NewMatchup(reg, [2]string{"a", "b"}, [2][]*cards.Card{deck, deck}, 0)
	if err != nil {
		t.Fatal(err)
	}
	side, err := RealKernel(tbl, nil, Config{})(m, 1)
	if err != nil {
		t.Fatalf("RealKernel: %v", err) // pre-fix: realkernel: "Cease" is not in table standard-meta
	}
	r := side.(*realSide)
	for i, want := range reported {
		if got := r.gname[i]; got != want.gorge {
			t.Errorf("gname[%d] = %q, want %q", i, got, want.gorge)
		}
		if r.tidx[i] == NoCardIndex || m.Table[r.tidx[i]] != want.gorge {
			t.Errorf("tidx[%d] = %d, %q is not at that index in the matchup table %v", i, r.tidx[i], want.gorge, m.Table)
		}
	}
}
