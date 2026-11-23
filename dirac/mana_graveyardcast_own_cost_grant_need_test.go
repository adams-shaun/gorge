package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestGraveyardCastNeedPricesAGrantsOwnCost pins the colour-ask hint of a
// flashback granted at its own cost (StFbOwnCost: Iroh, Grand Lotus lets a
// Lesson be cast from the graveyard for {1}): the cast pays the static's cost
// (fbGrantCost), so the hint names no pip of the card's {R} mana cost.
func TestGraveyardCastNeedPricesAGrantsOwnCost(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Iroh, Grand Lotus", Count: 2},
		{Name: "How to Start a Riot", Count: 2},
		{Name: "Mountain", Count: 8},
	}
	set := cardsettest.Set(t, "fbgrant")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
	sc.perm(id["Iroh, Grand Lotus"], 0)
	o := sc.grave(id["How to Start a Riot"], 0, 1)[0]
	if !sc.s.fbLive(sc.k, o) {
		t.Fatal("precondition: Iroh's grant is not live on the graveyard Lesson")
	}
	price := sc.s.fbGrantCost(sc.k, 0, o, sc.s.fbGranted(sc.k, o))
	if price == tbl.Cards[id["How to Start a Riot"]].Cost {
		t.Fatalf("precondition: the grant's own cost %v equals the card's mana cost", price)
	}
	a := MakeAction(AkCast, TCode(o), 0, 2)
	if _, _, ok := sc.s.playBaseCost(sc.k, a); ok {
		t.Fatal("precondition: playBaseCost prices the mode-2 cast, so the test does not reach the hint")
	}
	got, want := sc.s.playNeedHint(sc.k, a), grantNeedColours(price)
	if got != want {
		t.Fatalf("playNeedHint of the grant's own-cost cast = %v, want the grant cost %v's %v", got, price, want)
	}
}
