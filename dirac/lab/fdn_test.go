package lab_test

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/lab"
)

// fdnDeck is a 40-card deck of cards fdn plays: land, a creature, a second creature.
func fdnDeck(t *testing.T, name, land string, a, b string, commander string) lab.Deck {
	t.Helper()
	d := lab.Deck{Name: name, Cards: []cardset.Entry{{Name: land, Count: 17}, {Name: a, Count: 12}, {Name: b, Count: 10}}}
	if commander != "" {
		d.Cards = append(d.Cards, cardset.Entry{Name: commander, Count: 1})
		d.Commanders = []string{commander}
	}
	return d
}

func fdnDecks(t *testing.T, seats int, commander bool) []lab.Deck {
	cmd := func(s string) string {
		if commander {
			return s
		}
		return ""
	}
	all := []lab.Deck{
		fdnDeck(t, "R", "Mountain", "Goblin Boarders", "Searslicer Goblin", cmd("Frenzied Goblin")),
		fdnDeck(t, "G", "Forest", "Llanowar Elves", "Cat Collector", cmd("Lathril, Blade of the Elves")),
		fdnDeck(t, "W", "Plains", "Cat Collector", "Courageous Goblin", cmd("Goblin Bushwhacker")),
		fdnDeck(t, "B", "Swamp", "Goblin Boarders", "Frenzied Goblin", cmd("Searslicer Goblin")),
	}
	return all[:seats]
}

func TestFDNTwoSeats(t *testing.T) {
	cardsettest.Require(t, "fdn")
	cfg := lab.Config{Set: "fdn", Decks: fdnDecks(t, 2, false)}
	pols := []lab.Policy{lab.HBot(), lab.Random(1)}
	rs, sum, err := lab.Batch(cfg, pols, seeds(8), 2, 30000)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Failed != 0 || sum.Wins[0]+sum.Wins[1]+sum.Draws+sum.Stalls+sum.Truncated != 8 {
		t.Fatalf("%+v %+v", sum, rs)
	}
	if rs[0].Decks[0] != "R" {
		t.Fatalf("decks %v", rs[0].Decks)
	}
}

func TestFDNFourSeatPods(t *testing.T) {
	cardsettest.Require(t, "fdn")
	for _, commander := range []bool{false, true} {
		cfg := lab.Config{Set: "fdn", Decks: fdnDecks(t, 4, commander), Commander: commander}
		pols := []lab.Policy{lab.HBot(), lab.Random(1), lab.Random(2), lab.HBot()}
		_, sum, err := lab.Batch(cfg, pols, seeds(4), 2, 60000)
		if err != nil {
			t.Fatalf("commander=%v: %v", commander, err)
		}
		if sum.Failed != 0 || sum.Games != 4 || len(sum.Wins) != 4 {
			t.Fatalf("commander=%v: %+v", commander, sum)
		}
	}
}

func TestRefusedCardIsAnError(t *testing.T) {
	cardsettest.Require(t, "fdn")
	d := fdnDeck(t, "X", "Mountain", "No Such Card", "Goblin Boarders", "")
	if _, err := lab.Open(lab.Config{Set: "fdn", Decks: []lab.Deck{d, d}}); err == nil {
		t.Fatal("a card outside the set was accepted")
	}
}
