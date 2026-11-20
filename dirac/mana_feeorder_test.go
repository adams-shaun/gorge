package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// Baxter Building's {4},{T}: "four mana in any combination of colors" is a
// fee-carrying multi-unit gap. gorge's prefix search (potentialPrefixScript)
// runs its plain order first and, when that finds no script, a phased run in
// feeOrder: the covered funding first, then the multi-unit fee gaps, then the
// fee-free gaps and one-unit filters (scriptSearch.feeOrder). A board where
// the filter beside it (Surveillance Room's {1},{T} any colour) taps for {C}
// after Baxter in entry order is priced only by that run: the plain order
// spends its 160 nodes on Baxter's 64 allocations before the Room's {C} has
// floated the fourth unit of the fee.

func feeOrderScen(t *testing.T, bf []string) (*scen, Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Baxter Building", Count: 1}, {Name: "Surveillance Room", Count: 1},
		{Name: "Villainous Hideout", Count: 1}, {Name: "Island", Count: 2},
		{Name: "Hellcat, Undying Vigilante", Count: 1},
	}
	set := cardsettest.Synth(t, "std-msh")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, da[0], LockstepConfig(SurfaceFull))
	for _, n := range bf {
		sc.perm(id[n], 0)
	}
	return sc, sc.hand(id["Hellcat, Undying Vigilante"], 0)
}

func TestBaxterFilterPricedByFeeOrderRun(t *testing.T) {
	// the Room enters after Baxter: UU from the Islands, {C} from the Hideout
	// and from the Room's own {T} fund Baxter's {4}, which makes {G}{G}
	sc, hell := feeOrderScen(t, []string{"Island", "Island", "Villainous Hideout", "Baxter Building", "Surveillance Room"})
	if !sc.castOffered(0, hell) {
		t.Fatal("Hellcat ({G}{G}) is not offered from Baxter's four any-colour mana funded by Island, Island, Hideout and the Room's {C}")
	}
	// the control: the same sources with the Room first (the plain order finds it)
	sc, hell = feeOrderScen(t, []string{"Island", "Island", "Villainous Hideout", "Surveillance Room", "Baxter Building"})
	if !sc.castOffered(0, hell) {
		t.Fatal("control: Hellcat is not offered with the Room before Baxter")
	}
}

func TestBaxterFilterNeedsItsWholeFee(t *testing.T) {
	// three funders for a {4} fee: nothing pays it, in either entry order
	for _, bf := range [][]string{
		{"Island", "Island", "Baxter Building", "Surveillance Room"},
		{"Island", "Island", "Surveillance Room", "Baxter Building"},
	} {
		sc, hell := feeOrderScen(t, bf)
		if sc.castOffered(0, hell) {
			t.Fatalf("Hellcat is offered from %v: Baxter's {4} fee has only three funders", bf)
		}
	}
}
