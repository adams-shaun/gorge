package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// greatHallScen builds seat 0's board for Great Hall of the Citadel's
// restricted "{1}, {T}: Add two mana in any combination of colors" beside
// {T}: Add {C}: halls Great Halls beside the named basics, Kamahl, Pit Fighter
// ({4}{R}{R}, legendary) in hand.
func greatHallScen(t *testing.T, halls int, basics ...string) (*scen, Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Mountain", Count: 8}}
	set := cardsettest.Set(t, "restrictedmana")
	for _, n := range []string{"Plains", "Great Hall of the Citadel", "Kamahl, Pit Fighter"} {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the restrictedmana table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
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
	for _, b := range basics {
		sc.perm(id[b], 0)
	}
	for i := 0; i < halls; i++ {
		sc.perm(id["Great Hall of the Citadel"], 0)
	}
	return sc, sc.hand(id["Kamahl, Pit Fighter"], 0)
}

// TestGreatHallFeePairFundsALegendarySpell: four Great Halls, a Mountain and a
// Plains are six mana for Kamahl (CR 605.3b): two halls' {C} pay the other two's
// fees, whose four any-colour mana pay the {R}{R} and the rest.
func TestGreatHallFeePairFundsALegendarySpell(t *testing.T) {
	sc, kamahl := greatHallScen(t, 4, "Mountain", "Plains")
	if !offersCast(sc.s, kamahl) {
		t.Fatal("Kamahl not offered on four Great Halls, a Mountain and a Plains")
	}
}

// TestGreatHallFeePairCountsOneTapOnce: three Great Halls and two Mountains are
// five mana at most (one hall's {C} pays another's fee), short of Kamahl's six:
// a hall's {C} that paid a fee is not its own fee unit too.
func TestGreatHallFeePairCountsOneTapOnce(t *testing.T) {
	sc, kamahl := greatHallScen(t, 3, "Mountain", "Mountain")
	if offersCast(sc.s, kamahl) {
		t.Fatal("Kamahl offered on five mana: one Great Hall's tap counted twice")
	}
}
