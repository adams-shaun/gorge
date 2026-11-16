package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// randomDiscardAbility appends a zero-mana activated ability whose cost is
// Discard<n/Random> (CostDiscard beside Cost.DiscardRandom) to card fc.
func randomDiscardAbility(tbl *Table, fc CardID, n uint8) uint8 {
	tbl.Cards[fc].Abilities = append(tbl.Cards[fc].Abilities,
		kop.Ability{Kind: kop.AbActivated, Cost: kop.Cost{Flags: kop.CostDiscard, N: n, DiscardRandom: true},
			Prog: []kop.Instr{{Op: kop.OpGainEnergy, Obj: kop.RefController, A: kop.Const(1)}}})
	return uint8(len(tbl.Cards[fc].Abilities) - 1)
}

// randomDiscardScen puts one source on player 0's battlefield and cards more
// cards in their hand, the cost Discard<n/Random> on the source.
func randomDiscardScen(t *testing.T, n uint8, cards int) (sc *scen, src Obj, hand []Obj, ab uint8) {
	t.Helper()
	sc, id := newVerbScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 3}, {Name: "Plains", Count: 8}},
		func(tbl *Table, id map[string]CardID) {
			ab = randomDiscardAbility(tbl, id["Valley Floodcaller"], n)
		})
	fc := id["Valley Floodcaller"]
	if c := sc.k.t.Cards[fc].Abilities[ab].Cost; !c.DiscardRandom || c.Flags&kop.CostDiscard == 0 || c.N != n {
		t.Fatalf("precondition: the appended ability's cost is %+v", c)
	}
	src = sc.perm(fc, 0)
	for i := 0; i < cards; i++ {
		hand = append(hand, sc.hand(fc, 0))
	}
	if got := int(sc.s.pl[0].handN); got != cards {
		t.Fatalf("precondition: hand %d, want %d", got, cards)
	}
	return sc, src, hand, ab
}

// offered reports whether the decision offers the activation of ab on src.
func randomDiscardOffered(d Decision, src Obj, ab uint8) (Action, bool) {
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(src)) && a.Mode() == ab {
			return a, true
		}
	}
	return 0, false
}

// TestRandomDiscardCostMatchesGorgeRNG: a Discard<N/Random> activation cost
// discards the cards the engine RNG picks without asking: one draw per card
// over the hand in hand order, each pick removed (gorge discardAsk's Random
// arm, e.Rand(len(candidates)) over the shrinking list).
func TestRandomDiscardCostMatchesGorgeRNG(t *testing.T) {
	for _, tc := range []struct {
		name  string
		n     uint8
		cards int
	}{
		{"one of four", 1, 4},
		{"two of five", 2, 5},
		{"all of two", 2, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, src, hand, ab := randomDiscardScen(t, tc.n, tc.cards)
			// gorge's picks, from a copy of the seeded generator
			probe := sc.s.rng
			left := append([]Obj(nil), hand...)
			var want []Obj
			for i := 0; i < int(tc.n); i++ {
				j := probe.IntN(len(left))
				want = append(want, left[j])
				left = append(left[:j], left[j+1:]...)
			}
			before := sc.s.rng.Draws
			a, ok := randomDiscardOffered(sc.s.Step(), src, ab)
			if !ok {
				t.Fatal("the activation is not offered")
			}
			sc.s.Apply(a)
			if d := sc.s.Step(); d.Kind != DkPriority {
				t.Fatalf("decision kind %d after paying, want priority (a random discard asks nothing)", d.Kind)
			}
			if got := sc.s.rng.Draws - before; got != uint64(tc.n) {
				t.Errorf("%d RNG draws, want %d (one per card)", got, tc.n)
			}
			for _, o := range want {
				if z := sc.s.objs[o].zone; z != kop.ZoneGraveyard {
					t.Errorf("gorge's pick %d is in zone %d, want the graveyard", o, z)
				}
			}
			for _, o := range left {
				if z := sc.s.objs[o].zone; z != kop.ZoneHand {
					t.Errorf("unpicked card %d is in zone %d, want the hand", o, z)
				}
			}
			if got := int(sc.s.pl[0].handN); got != tc.cards-int(tc.n) {
				t.Errorf("hand %d after the cost, want %d", got, tc.cards-int(tc.n))
			}
		})
	}
}

// TestRandomDiscardCostWithheldOnShortHand: a hand of fewer than N cards
// withholds the activation (gorge's unpayable Discard part), and the same
// ability is offered once the hand holds N.
func TestRandomDiscardCostWithheldOnShortHand(t *testing.T) {
	for _, tc := range []struct {
		name    string
		n       uint8
		cards   int
		offered bool
	}{
		{"empty hand", 1, 0, false},
		{"one card for two", 2, 1, false},
		{"exactly enough", 2, 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, src, _, ab := randomDiscardScen(t, tc.n, tc.cards)
			if _, ok := randomDiscardOffered(sc.s.Step(), src, ab); ok != tc.offered {
				t.Fatalf("activation offered = %v with %d cards for Discard<%d/Random>, want %v", ok, tc.cards, tc.n, tc.offered)
			}
		})
	}
}
