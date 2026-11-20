package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// feeOrderGapScen: p0 has a Viridescent Bog beside a Baxter Building, four
// Mountains and a Forest: two fee-carrying census filters (Bog's {1},{T}
// two-unit ability and Baxter's {4},{T} four-unit one, mana_filter.go
// filtFeeOrder), with The Unbeatable Squirrel Girl ({1}{G}{G}{G}) and
// Blasphemous Edict in hand.
func feeOrderGapScen(t *testing.T) (s *State, k *prep, sg, be Obj) {
	t.Helper()
	set := cardsettest.Set(t, "feegaps")
	deck := []cardset.Entry{
		{Name: "Baxter Building", Count: 1},
		{Name: "Viridescent Bog", Count: 1},
		{Name: "The Unbeatable Squirrel Girl", Count: 1},
		{Name: "Blasphemous Edict", Count: 1},
		{Name: "Mountain", Count: 8},
		{Name: "Forest", Count: 8},
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
	sc.perm(id["Viridescent Bog"], 0)
	sc.perm(id["Baxter Building"], 0)
	for i := 0; i < 4; i++ {
		sc.perm(id["Mountain"], 0)
	}
	sc.perm(id["Forest"], 0)
	sg = sc.hand(id["The Unbeatable Squirrel Girl"], 0)
	be = sc.hand(id["Blasphemous Edict"], 0)
	return sc.s, sc.k, sg, be
}

// TestFilterFeeOrderSkipsAltCost: the feeOrder scope is gorge's plain cast
// (potentialPrefixScript's build(true), potential_script.go: a cast whose
// Mode is empty and AltCostIndex 0). A dirac alternative-cost cast is mode 0
// with target castAltTgt (cast_altfree.go), so the scope test must read the
// target as well as the mode.
func TestFilterFeeOrderSkipsAltCost(t *testing.T) {
	s, k, sg, be := feeOrderGapScen(t)
	gaps := s.scriptFilt(k, 0)
	plain := MakeAction(AkCast, TCode(sg), 0, 0)
	if !s.filtFeeOrder(k, 0, plain, gaps) {
		t.Fatal("precondition: the plain cast is not in the feeOrder scope")
	}
	alt := MakeAction(AkCast, TCode(be), castAltTgt, 0)
	if s.filtFeeOrder(k, 0, alt, gaps) {
		t.Fatal("the alternative-cost cast entered the plain cast's feeOrder scope")
	}
}
