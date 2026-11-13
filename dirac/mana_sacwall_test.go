package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEveryUnitNeededPlanIsFound: a payment whose units each cover a pip no
// other unit can is a plan however much mana they make together. Cycling
// Twisted Landscape ({B}{R}{G}) from a Swamp, a Tinder Wall (sacrifice: {R}{R})
// and an Overgrown Battlement ({G} for each creature with defender: three here)
// makes 6 mana for a 3-mana cost, and every one of the three is needed. The
// planner's walk stopped taking units once the total reached the cost plus the
// largest unit, so it never tried the Battlement after the other two, found no
// plan, and the payment panicked "autopay found no plan" after the offer.
func TestEveryUnitNeededPlanIsFound(t *testing.T) {
	deck := []cardset.Entry{{Name: "Twisted Landscape", Count: 1}, {Name: "Swamp", Count: 10},
		{Name: "Tinder Wall", Count: 1}, {Name: "Overgrown Battlement", Count: 1}, {Name: "Wall of Roots", Count: 1}}
	set := cardsettest.Set(t, "pauper")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for _, cfg := range []Config{{NoElide: true}, {NoElide: true, GorgeLowering: true, GorgeAutopaySurface: true}} {
		sc := newScenTableLib(t, tbl, id["Swamp"], cfg)
		for _, n := range []string{"Swamp", "Tinder Wall", "Overgrown Battlement", "Wall of Roots"} {
			sc.perm(id[n], 0)
		}
		tl := sc.hand(id["Twisted Landscape"], 0)
		d := sc.s.Step()
		offered := false
		for _, a := range d.Options[:d.N] {
			offered = offered || a.Kind() == AkActivate && a.Source() == TCode(uint8(tl))
		}
		if !offered {
			t.Fatalf("cfg %+v: the Landscape's cycling is not offered: B, R (the Wall's sacrifice) and G are all there", cfg)
		}
		sc.do(t, AkActivate, TCode(tl), 0)
		sc.s.Step() // the payment: no "autopay found no plan"
		if z := sc.s.objs[tl].zone; z != kop.ZoneGraveyard {
			t.Fatalf("cfg %+v: the cycled card is in zone %d, want the graveyard", cfg, z)
		}
	}
}
