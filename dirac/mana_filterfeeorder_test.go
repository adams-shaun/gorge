package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFilterFeeOrderSkipsAltCost: the feeOrder scope is gorge's plain cast
// (potentialPrefixScript's build(true), potential_script.go: a cast whose
// Mode is empty and AltCostIndex 0). A dirac alternative-cost cast is mode 0
// with target castAltTgt (cast_altfree.go, the oracle's AltCostIndex 1 cast),
// so the scope test must read the target as well as the mode. Baxter
// Building's {4},{T} four-unit filter (the msh pair-102 board) is the
// fee-carrying census gap the scope needs.
func TestFilterFeeOrderSkipsAltCost(t *testing.T) {
	set := cardsettest.Set(t, "manasecond")
	deck := []cardset.Entry{
		{Name: "Baxter Building", Count: 1},
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Wastes", Count: 8},
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Wastes"], Config{NoElide: true})
	baxter := sc.perm(id["Baxter Building"], 0)
	bears := sc.hand(id["Grizzly Bears"], 0)
	s, k := sc.s, sc.k
	gaps := s.scriptFilt(k, 0)
	if !gaps.has(sc.slot(baxter)) {
		t.Fatal("precondition: Baxter Building is no census filter gap")
	}
	plain := MakeAction(AkCast, TCode(bears), 0, 0)
	if !s.filtFeeOrder(k, 0, plain, gaps) {
		t.Fatal("precondition: the plain cast is not in the feeOrder scope")
	}
	alt := MakeAction(AkCast, TCode(bears), castAltTgt, 0)
	if s.filtFeeOrder(k, 0, alt, gaps) {
		t.Fatal("the alternative-cost cast entered the plain cast's feeOrder scope")
	}
}
