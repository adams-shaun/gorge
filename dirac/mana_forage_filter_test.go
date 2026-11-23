package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// CR 605.3a, 601.2g: Thornvault Forager's "{T}, Forage: add two mana in any
// combination of colors" beside Hidden Grotto's "{1},{T}: add one mana of any color".
// gorge's one prefix search activates both gaps: a payable forage (a Food, or three
// graveyard cards) pays Parting Gust's {W}{W} with the Plains (the filter search's
// forage step), and a forage that cannot be paid is no census gap, so the Grotto's
// filter, funded by a Forest, still pays it.
func TestForageSourceBesideFilterFundsPlay(t *testing.T) {
	names := []string{"Parting Gust", "Hidden Grotto", "Rockface Village", "Plains", "Forest", "Thornvault Forager", "Innkeeper's Talent", "Bumbleflower's Sharepot"}
	set := cardsettest.Set(t, "blb")
	var deck []cardset.Entry
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not in the table", n)
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
	food := CardID(0xFFFF)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "c_a_food_sac" {
			food = CardID(i)
		}
	}
	if food == 0xFFFF {
		t.Fatal("no Food token in the table")
	}
	type shape struct {
		with  []string
		foods int
		grave int
	}
	all := []string{"Hidden Grotto", "Forest", "Rockface Village", "Forest"}
	for _, sh := range []shape{{nil, 1, 0}, {[]string{"Hidden Grotto"}, 1, 0}, {[]string{"Hidden Grotto"}, 0, 3}, {[]string{"Hidden Grotto"}, 1, 3}, {all, 1, 0}, {all, 0, 3}, {all, 1, 4}, {[]string{"Hidden Grotto", "Forest"}, 0, 2}, {all, 0, 2}, {all, 0, 0}} {
		with := sh.with
		sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
		sc.perm(id["Plains"], 0)
		sc.perm(id["Thornvault Forager"], 0)
		for i := 0; i < sh.foods; i++ {
			sc.perm(food, 0)
		}
		for i := 0; i < sh.grave; i++ {
			sc.s.place(sc.obj(id["Plains"], 0), kop.ZoneGraveyard, false)
		}
		for _, w := range with {
			sc.perm(id[w], 0)
		}
		gust := sc.hand(id["Parting Gust"], 0)
		d := sc.s.Step()
		t.Logf("%+v: cast offered=%v", sh, castOffered(&d, gust))
		if !castOffered(&d, gust) {
			t.Errorf("%+v: Parting Gust not offered", sh)
		}
	}
}
