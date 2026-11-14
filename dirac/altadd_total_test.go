package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// CR 601.2f: an either-or additional cost's mana part joins the spell's cost
// before the reductions take their generic off the total, and the offer prices
// that total. With too little life for the pay-life part, Redirect Lightning's
// {2} part makes {2}{R}; Uncle Iroh's {1} off the total leaves {1}{R}, which a
// Mountain and a Plains pay, though the same {1} off the spell's {R} alone
// would be lost.
func TestAltAddOfferPricesTheReducedTotal(t *testing.T) {
	deck := []cardset.Entry{{Name: "Uncle Iroh", Count: 1}, {Name: "Redirect Lightning", Count: 1},
		{Name: "Combustion Technique", Count: 1}, {Name: "Ostrich-Horse", Count: 1}, {Name: "Mountain", Count: 20}, {Name: "Plains", Count: 20}}
	set := cardsettest.Synth(t, "std-tla")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the std-tla table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for _, tc := range []struct {
		name   string
		iroh   bool
		lands  int
		offers bool
	}{{"reduced to two mana", true, 2, true}, {"unreduced needs three", false, 2, false}, {"reduced, one land short", true, 1, false}} {
		t.Run(tc.name, func(t *testing.T) {
			sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
			target := sc.perm(id["Ostrich-Horse"], 0)
			if tc.iroh {
				sc.perm(id["Uncle Iroh"], 0)
			}
			sc.perm(id["Mountain"], 0)
			if tc.lands == 2 {
				sc.perm(id["Plains"], 0)
			}
			rl := sc.hand(id["Redirect Lightning"], 0)
			sc.s.pl[0].life = 4
			spell := sc.crimeStackSpell(id["Combustion Technique"], 1)
			e := &sc.s.stack[sc.s.sp-1]
			_ = spell
			e.nt, e.tgt[0] = 1, TCode(target)
			if got := offersCast(sc.s, rl); got != tc.offers {
				t.Fatalf("Redirect Lightning offered = %v, want %v", got, tc.offers)
			}
		})
	}
}
