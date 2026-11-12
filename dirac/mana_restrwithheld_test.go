package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRestrWithheldManaCountsAnUnspendableCensusSource: Heartwood Crafter's
// "{T}: Add {C}. This mana can't be spent to cast spells from your hand" is a unit
// of gorge's census whether or not the payment may spend it, so the census total
// an {X} witness divides by X holds its mana for a hand cast and for a cast from
// exile alike. Devastating Onslaught {X}{X}{R} under a five-mana reduction beside
// Spider Manifestation and four lands is searched up to 6/2 = 3, not the 5/2 = 2 of
// the mana the hand cast can spend, and the witness taps one source more (CR 601.2b).
func TestRestrWithheldManaCountsAnUnspendableCensusSource(t *testing.T) {
	set := cardset.AllowUnverified(cardsettest.Set(t, "standard-meta"))
	deck := []cardset.Entry{{Name: "Heartwood Crafter", Count: 1}, {Name: "Forest", Count: 1}, {Name: "Devastating Onslaught", Count: 1}, {Name: "Island", Count: 16}}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	for _, n := range []string{"Heartwood Crafter", "Forest", "Devastating Onslaught"} {
		if _, ok := id[n]; !ok {
			t.Fatalf("%s is not in the table", n)
		}
	}
	sc.perm(id["Heartwood Crafter"], 0)
	sc.perm(id["Forest"], 0)
	s, k := sc.s, sc.k
	if k.gen == nil || k.gen.flags[id["Heartwood Crafter"]]&gfRestrSrc == 0 {
		t.Fatal("Heartwood Crafter is no restricted source of the table")
	}
	hand := payDesc{class: pdSpell, card: id["Devastating Onslaught"], src: NoSlot, og: restrOrigin{from: kop.ZoneHand, cast: true, byYou: true}}
	exile := hand
	exile.og.from = kop.ZoneExile
	if got := s.restrWithheldMana(k, 0, exile); got != 0 {
		t.Fatalf("a cast from exile may spend the Crafter's {C}, yet %d mana is withheld", got) // the precondition: the withheld count follows the payment
	}
	if got := s.restrWithheldMana(k, 0, hand); got != 1 {
		t.Errorf("withheld mana of a hand cast = %d, want 1 (the Crafter's {C}; the Forest is spendable and counted apart)", got)
	}

	var cost kop.ManaCost
	cost.X, cost.XRed = 2, 5
	cost.Pips[kop.R] = 1
	var av avail
	av.addUnits(1<<kop.R, 1)
	av.addUnits(1<<kop.G, 4)
	if got := restrWitnessXPlus(2, av, nil, &cost, 0); got != 2 {
		t.Fatalf("witness X without the withheld mana = %d, want 2 (5/2)", got) // the precondition: the reduced cost pays past it
	}
	if got := restrWitnessXPlus(2, av, nil, &cost, 1); got != 3 {
		t.Errorf("witness X with the withheld mana = %d, want 3 (6/2)", got)
	}
}
