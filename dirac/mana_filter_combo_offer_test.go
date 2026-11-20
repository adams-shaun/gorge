package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestBaxterComboFilterFundsWhiteCastBesideDualLands: Baxter Building's
// {4},{T} any-combination filter is the only white source (three Forests,
// Stark Industries' {U}/{R} and Pym Technologies' {G}/{U} fund its fee), so
// The Sentry's {3}{W} is affordable through the filter's WWWW (CR 605.3a,
// 601.2g), and gorge offers the cast.
func TestBaxterComboFilterFundsWhiteCastBesideDualLands(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Baxter Building", Count: 1}, {Name: "Stark Industries", Count: 1},
		{Name: "Pym Technologies", Count: 1}, {Name: "Forest", Count: 3},
		{Name: "The Sentry, Golden Guardian", Count: 1}, {Name: "Crowd of True Believers", Count: 1},
	}
	tbl, da, _, err := cardsettest.Set(t, "combooffer").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true, GorgeAutopaySurface: false})
	sc.perm(id["Forest"], 0)
	stark := sc.perm(id["Stark Industries"], 0)
	sc.perm(id["Forest"], 0)
	pym := sc.perm(id["Pym Technologies"], 0)
	sc.perm(id["Baxter Building"], 0)
	sc.perm(id["Forest"], 0)
	sc.perm(id["Crowd of True Believers"], 0)
	for _, o := range []Obj{stark, pym} { // both entered tapped; the game's had untapped
		sc.s.perms[sc.slot(o)].flags &^= pfTapped
	}
	sentry := sc.hand(id["The Sentry, Golden Guardian"], 0)
	if acts := sc.castOptions(sentry); len(acts) == 0 {
		t.Fatalf("The Sentry's cast is not offered: Baxter Building's filter should make {W}{W}{W}{W}")
	}
}
