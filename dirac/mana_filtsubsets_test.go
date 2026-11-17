package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// subsetScen: p0 in main 1 with the given untapped Plains and Three Tree
// Mascots whose "{1}: Add one mana of any color" is raised to {2}, a filter
// dearer than the one mana it makes (Celestial Prism's {2},{T}), and Plumecreed
// Escort ({1}{U}) in hand.
func subsetScen(t *testing.T, mascots, plains int) (*scen, Obj) {
	t.Helper()
	sc, id := newBLBScenTweak(t, []cardset.Entry{{Name: "Three Tree Mascot", Count: 2}, {Name: "Plumecreed Escort", Count: 1},
		{Name: "Plains", Count: 8}}, func(tbl *kop.Table, id map[string]CardID) {
		cd := &tbl.Cards[id["Three Tree Mascot"]]
		cd.Abilities = append([]kop.Ability(nil), cd.Abilities...)
		for i := range cd.Abilities {
			if a := &cd.Abilities[i]; a.Kind == kop.AbMana {
				a.Cost.Mana.Generic = 2
			}
		}
	})
	for i := 0; i < plains; i++ {
		sc.perm(id["Plains"], 0)
	}
	for i := 0; i < mascots; i++ {
		sc.perm(id["Three Tree Mascot"], 0)
	}
	if !sc.k.gen.anyFilt || !sc.k.gen.has(id["Three Tree Mascot"], gfFilt) {
		t.Fatal("the raised Mascot is not a filter of the prepared table")
	}
	return sc, sc.hand(id["Plumecreed Escort"], 0)
}

// TestFilterSubsetPaysWhereAllFiltersCannot: gorge's relaxed proof runs the
// verdict for each subset of the census's paid filters (paymentPlanRelaxProof)
// and calls a play insufficient only when every run is. With a filter dearer
// than the mana it makes, the run that charges every filter is not the most
// permissive: three Plains and two such Mascots pay {1}{U} through one Mascot
// (3 + 1 - 2 = 2 mana, one of any colour) though both together leave 1. Two
// Plains pay a fee and have nothing left to cast with.
func TestFilterSubsetPaysWhereAllFiltersCannot(t *testing.T) {
	for _, c := range []struct {
		name            string
		mascots, plains int
		want            int
	}{
		{"one Mascot, three Plains", 1, 3, 1},
		{"two Mascots, three Plains", 2, 3, 1},
		{"two Mascots, four Plains", 2, 4, 1},
		{"two Mascots, two Plains", 2, 2, 0},
	} {
		sc, escort := subsetScen(t, c.mascots, c.plains)
		if got := len(sc.castOptions(escort)); got != c.want {
			t.Errorf("%s: %d Plumecreed Escort cast options, want %d", c.name, got, c.want)
		}
	}
}
