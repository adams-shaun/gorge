package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

func lifeGapAutopayScen(t *testing.T, ogres, mountains int, life int16) (sc *scen, stone Obj, stoneID CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Treasonous Ogre", Count: 2}, {Name: "Mind Stone", Count: 1}, {Name: "Mountain", Count: 17}}
	tbl, _, _, err := cardsettest.Set(t, "mbc").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc = newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true, GorgeAutopaySurface: true})
	for i := 0; i < ogres; i++ {
		sc.perm(id["Treasonous Ogre"], 0)
	}
	for i := 0; i < mountains; i++ {
		sc.perm(id["Mountain"], 0)
	}
	stone = sc.hand(id["Mind Stone"], 0)
	sc.s.pl[0].life = life
	return sc, stone, id["Mind Stone"]
}

// Two Treasonous Ogres at 4 life fund one 3-life activation, not two: on the
// autopay surface the {2} Mind Stone must not be offered (the census's life
// units share the life total), and a single Ogre beside a Mountain funds it
// and pays one activation, never overdrawing the life total.
func TestLifeGapAutopayBudgetNotOffered(t *testing.T) {
	sc, stone, _ := lifeGapAutopayScen(t, 2, 0, 4)
	if offersCast(sc.s, stone) {
		t.Fatal("the {2} cast is offered with one payable Ogre activation (autopay)")
	}
}

func TestLifeGapAutopayBudgetControlPaysOneActivation(t *testing.T) {
	sc, stone, _ := lifeGapAutopayScen(t, 1, 1, 4)
	if !offersCast(sc.s, stone) {
		t.Fatal("precondition: Mountain plus one Ogre at 4 life does not offer the {2} cast")
	}
	sc.do(t, AkCast, TCode(stone), scAny)
	sc.s.Step() // the lazy cast pays at the next decision
	if got := sc.s.pl[0].life; got != 1 {
		t.Fatalf("life %d after the cast, want 1 (one 3-life activation beside the Mountain)", got)
	}
}
