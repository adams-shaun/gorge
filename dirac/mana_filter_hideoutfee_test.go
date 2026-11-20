package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// CR 605.3b, 106.6: Villainous Hideout's {C} mode is unrestricted, so it pays
// Baxter Building's {4} beside three Swamps; the four mana of any colours then
// pay Death to Our Enemies' {2}{R}. Only its any-colour mode is Villain-only.
func TestHideoutColourlessPaysBaxterFee(t *testing.T) {
	names := []string{"Death to Our Enemies", "Baxter Building", "Villainous Hideout"}
	set := cardsettest.Synth(t, "std-msh")
	var deck []cardset.Entry
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not in the std-msh table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	deck = append(deck, cardset.Entry{Name: "Swamp", Count: 1})
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for _, swamps := range []int{2, 3} {
		sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
		sc.perm(id["Baxter Building"], 0)
		sc.perm(id["Villainous Hideout"], 0)
		for i := 0; i < swamps; i++ {
			sc.perm(id["Swamp"], 0)
		}
		death := sc.hand(id["Death to Our Enemies"], 0)
		d := sc.s.Step()
		if got, want := castOffered(&d, death), swamps == 3; got != want {
			t.Errorf("%d Swamps: cast offered=%v, want %v", swamps, got, want)
		}
	}
}
