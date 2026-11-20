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
	set := cardsettest.Synth(t, "std-msh")
	deck := []cardset.Entry{
		{Name: "Baxter Building", Count: 1},
		{Name: "The Unbeatable Squirrel Girl", Count: 1},
		{Name: "Mountain", Count: 8},
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
	baxter := sc.perm(id["Baxter Building"], 0)
	sg := sc.hand(id["The Unbeatable Squirrel Girl"], 0)
	s, k := sc.s, sc.k
	gaps := s.scriptFilt(k, 0)
	if !gaps.has(sc.slot(baxter)) {
		t.Fatal("precondition: Baxter Building is no census filter gap")
	}
	plain := MakeAction(AkCast, TCode(sg), 0, 0)
	if !s.filtFeeOrder(k, 0, plain, gaps) {
		t.Fatal("precondition: the plain cast is not in the feeOrder scope")
	}
	alt := MakeAction(AkCast, TCode(sg), castAltTgt, 0)
	if s.filtFeeOrder(k, 0, alt, gaps) {
		t.Fatal("the alternative-cost cast entered the plain cast's feeOrder scope")
	}
}
