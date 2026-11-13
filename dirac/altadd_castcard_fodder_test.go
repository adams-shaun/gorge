package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAltAddGatesSkipCastCardAsExileFodder (CR 601.2a, 601.2h): the card cast
// from the graveyard is on the stack while its either-or additional cost is
// paid, so it is not one of the graveyard cards the free part exiles. With one
// other graveyard card the exile-two part is unpayable and the mana part
// ({2}) is what the pool must pay; two others keep the free part payable.
func TestAltAddGatesSkipCastCardAsExileFodder(t *testing.T) {
	for _, tc := range []struct {
		others int
		want   bool
	}{{1, false}, {2, true}} {
		carrier := kop.CardDef{Name: "Test GraveEitherOr", Types: kop.TSorcery,
			Cost: kop.ManaCost{Generic: 1}, SpellCost: kop.Cost{Flags: kop.CostExileGrave, N: 2},
			AltAddCost: kop.Cost{Mana: kop.ManaCost{Generic: 2}}, HasAltAdd: true}
		sc, o := mpocScen(t, "tapall", carrier, Config{}, true, "Island", "Island")
		s, k := sc.s, sc.k
		s.objs[o].zone = kop.ZoneNone
		s.place(o, kop.ZoneGraveyard, false)
		s.g.may[0].ver = s.objs[o].ver
		if s.objs[o].zone != kop.ZoneGraveyard {
			t.Fatalf("others=%d: the cast card is in zone %d, not the graveyard", tc.others, s.objs[o].zone)
		}
		c := s.objs[o].card
		for i := 0; i < tc.others; i++ {
			f := sc.obj(c, 0)
			s.place(f, kop.ZoneGraveyard, false)
		}
		if !k.t.Cards[c].HasAltAdd {
			t.Fatalf("others=%d: the carrier has no either-or additional cost", tc.others)
		}
		var av avail
		s.availFor(k, 0, &av)
		cost := k.cards[c].cost
		if got := s.altAddCastable(k, c, 0, o, &cost, &av); got != tc.want {
			t.Errorf("others=%d: altAddCastable = %v, want %v", tc.others, got, tc.want)
		}
		s.pl[0].pool[kop.U] = 1
		if got := s.FeatPoolPays(0, o, 0); got != tc.want {
			t.Errorf("others=%d: FeatPoolPays({U}) = %v, want %v", tc.others, got, tc.want)
		}
	}
}
