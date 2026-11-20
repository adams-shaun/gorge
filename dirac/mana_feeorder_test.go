package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFeeOrderRunPricesFilterBesideRestrictedFunder: Baxter Building's
// "{4}, {T}: Add four mana in any combination" beside three Mountains and
// either a fourth Mountain or a Villainous Hideout pays Colleen Wing's {1}{W}.
// The Hideout's restricted any-colour ability (Villain spells only) gives the
// plain prefix search seven answers a tap, so it spends its whole node budget
// before the funding line; gorge's phased run walks the funding first and the
// fee-carrying gap next (scriptSearch.feeOrder), and the kernel's offer must
// follow.
func TestFeeOrderRunPricesFilterBesideRestrictedFunder(t *testing.T) {
	for _, fourth := range []string{"Mountain", "Villainous Hideout"} {
		deck := []cardset.Entry{{Name: "Baxter Building", Count: 1}, {Name: "Colleen Wing, Street Samurai", Count: 1},
			{Name: "Mountain", Count: 8}, {Name: "Villainous Hideout", Count: 1}}
		tbl, da, _, err := cardsettest.Synth(t, "std-msh").Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		id := map[string]CardID{}
		for _, c := range da {
			id[tbl.Cards[c].Name] = c
		}
		sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
		sc.perm(id["Baxter Building"], 0)
		for _, n := range []string{"Mountain", "Mountain", "Mountain", fourth} {
			sc.perm(id[n], 0)
		}
		colleen := sc.hand(id["Colleen Wing, Street Samurai"], 0)
		d := sc.s.Step()
		offered := false
		for _, a := range d.Options[:d.N] {
			offered = offered || a.Kind() == AkCast && Obj(a.Source()) == colleen
		}
		if !offered {
			t.Errorf("fourth land %s: Colleen Wing's cast is not offered through Baxter Building's filter", fourth)
		}
	}
}
