package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// Lupinflower Village's {1}{W}, {T}, Sacrifice ability is affordable when the
// only white source is a filter land (Hidden Grotto: {1}, {T}: one mana of any
// colour) funded by a Mountain (CR 602.2b, 605.3a): gorge offers it.
func TestLupinflowerOfferedOverFilterLand(t *testing.T) {
	deck := []cardset.Entry{{Name: "Lupinflower Village", Count: 1}, {Name: "Hidden Grotto", Count: 1},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Uncharted Haven", Count: 1}, {Name: "Thornvault Forager", Count: 2}}
	set := cardsettest.Set(t, "blb")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true, GorgeAutopaySurface: true})
	v := sc.perm(id["Lupinflower Village"], 0)
	sc.perm(id["Hidden Grotto"], 0)
	for i := 0; i < 3; i++ {
		sc.perm(id["Mountain"], 0)
	}
	sc.perm(id["Forest"], 0)
	sc.perm(id["Forest"], 0)
	sc.perm(id["Uncharted Haven"], 0)
	sc.perm(id["Thornvault Forager"], 0)
	d0 := sc.s.Step()
	if d0.Kind == DkChoose { // Uncharted Haven's colour: black, as in the game
		sc.s.Apply(d0.Options[2])
	}
	if !sc.activateOffered(v, 2) {
		t.Fatal("Lupinflower Village's {1}{W} sacrifice ability is not offered over Hidden Grotto's filter")
	}
}
