package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// On the autopay surface a plain cast only a life-only source completes is a
// plan gorge's planner finds in phase 2 (the Ogre's PayLife<3> is a last-resort
// alternative), refused only when the life paid would reach the payer's life
// (pay/search.go walk's lethal guard). {4}{R} Rapacious Dragon beside four
// Mountains and a Treasonous Ogre: offered at 4 life, not at 3.
func TestLifeGapPlansPlainCastOnAutopaySurface(t *testing.T) {
	deck := []cardset.Entry{{Name: "Treasonous Ogre", Count: 1}, {Name: "Rapacious Dragon", Count: 1}, {Name: "Mountain", Count: 18}}
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
	offered := func(life int) bool {
		sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true, GorgeAutopaySurface: true})
		sc.lands(0, id["Mountain"], 4)
		sc.perm(id["Treasonous Ogre"], 0)
		dragon := sc.hand(id["Rapacious Dragon"], 0)
		s := sc.s
		s.pl[0].life = int16(life)
		if mv := sc.k.t.Cards[id["Rapacious Dragon"]].Cost.MV(); mv != 5 {
			t.Fatalf("precondition: Rapacious Dragon costs {%d}, want five mana", mv)
		}
		if !s.lifeGapMask(sc.k, 0).any() {
			t.Fatalf("precondition: the Ogre is not a payable life gap at %d life", life)
		}
		d := s.Step()
		for i := 0; i < int(d.N); i++ {
			if o := d.Options[i]; o.Kind() == AkCast && o.Source() == TCode(dragon) {
				return true
			}
		}
		return false
	}
	if !offered(4) {
		t.Fatal("the Dragon is not offered with four Mountains and an Ogre at 4 life")
	}
	if offered(3) {
		t.Fatal("the Dragon is offered at 3 life: paying the Ogre's 3 life reaches the payer's life, the planner's lethal guard refuses it")
	}
}
