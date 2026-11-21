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
// floated the fourth unit of the fee (std-msh pair 138: Hellcat from Island,
// Island, Villainous Hideout, Baxter Building and Surveillance Room).

func feeOrderScen(t *testing.T, bf []string) (*scen, Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Baxter Building", Count: 1}, {Name: "Surveillance Room", Count: 1},
		{Name: "Wastes", Count: 1}, {Name: "Breya, Etherium Shaper", Count: 1},
	}
	set := cardsettest.Set(t, "manasecond")
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
	return sc, sc.hand(id["Breya, Etherium Shaper"], 0)
}

func TestBaxterFilterPricedByFeeOrderRun(t *testing.T) {
	// the Room enters after Baxter: {C} from three Wastes and from the Room's
	// own {T} fund Baxter's {4}, which makes Breya's {W}{U}{B}{R}
	sc, breya := feeOrderScen(t, []string{"Wastes", "Wastes", "Wastes", "Baxter Building", "Surveillance Room"})
	if !sc.castOffered(0, breya) {
		t.Fatal("Breya is not offered from Baxter's four any-colour mana funded by three Wastes and the Room's {C}")
	}
	// the control: the same sources with the Room first (the plain order finds it)
	sc, breya = feeOrderScen(t, []string{"Wastes", "Wastes", "Wastes", "Surveillance Room", "Baxter Building"})
	if !sc.castOffered(0, breya) {
		t.Fatal("control: Breya is not offered with the Room before Baxter")
	}
}

func TestBaxterFilterNeedsItsWholeFee(t *testing.T) {
	// three funders for a {4} fee: nothing pays it, in either entry order
	for _, bf := range [][]string{
		{"Wastes", "Wastes", "Baxter Building", "Surveillance Room"},
		{"Wastes", "Wastes", "Surveillance Room", "Baxter Building"},
	} {
		sc, breya := feeOrderScen(t, bf)
		if sc.castOffered(0, breya) {
			t.Fatalf("Breya is offered from %v: Baxter's {4} fee has only three funders", bf)
		}
	}
}
