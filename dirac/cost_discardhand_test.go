package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// discardHandAbility appends a zero-mana activated ability whose cost is
// Cost.DiscardHand to card fc and returns its ability index.
func discardHandAbility(t *testing.T, tbl *Table, fc CardID) uint8 {
	t.Helper()
	tbl.Cards[fc].Abilities = append(tbl.Cards[fc].Abilities,
		kop.Ability{Kind: kop.AbActivated, Cost: kop.Cost{DiscardHand: true},
			Prog: []kop.Instr{{Op: kop.OpGainEnergy, Obj: kop.RefController, A: kop.Const(1)}}})
	return uint8(len(tbl.Cards[fc].Abilities) - 1)
}

// TestDiscardHandActCost: a Cost.DiscardHand activation discards every card
// in the payer's hand in hand order as it is paid, and an empty hand still
// pays with nothing (CR 118.3, 602.2b; gorge pay.DiscardCandidates'
// whole-hand spec).
func TestDiscardHandActCost(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cards int
	}{
		{"three cards", 3},
		{"empty hand", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var ab uint8
			sc, id := newVerbScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 3}, {Name: "Plains", Count: 8}},
				func(tbl *Table, id map[string]CardID) {
					ab = discardHandAbility(t, tbl, id["Valley Floodcaller"])
				})
			fc := id["Valley Floodcaller"]
			if !sc.k.t.Cards[fc].Abilities[ab].Cost.DiscardHand {
				t.Fatal("precondition: the appended ability does not carry Cost.DiscardHand")
			}
			src := sc.perm(fc, 0)
			hand := make([]Obj, tc.cards)
			for i := range hand {
				hand[i] = sc.hand(fc, 0)
			}
			if n := int(sc.s.pl[0].handN); n != tc.cards {
				t.Fatalf("precondition: hand %d, want %d", n, tc.cards)
			}
			d := sc.s.Step()
			found := false
			for i := 0; i < int(d.N); i++ {
				if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(src)) && a.Mode() == ab {
					sc.s.Apply(a)
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("DiscardHand activation not offered: decision %d player %d opts %v", d.Kind, d.Player, d.Options[:d.N])
			}
			if d := sc.s.Step(); d.Kind != DkPriority {
				t.Fatalf("decision kind %d after paying, want priority", d.Kind)
			}
			if n := int(sc.s.pl[0].handN); n != 0 {
				t.Fatalf("hand %d after the cost, want 0", n)
			}
			for _, o := range hand {
				if z := sc.s.objs[o].zone; z != kop.ZoneGraveyard {
					t.Fatalf("discarded card %d is in zone %d, want the graveyard", o, z)
				}
			}
			sc.passBoth(t)
			sc.s.Step()
			if e := sc.s.Energy(0); e != 1 {
				t.Fatalf("energy %d: the ability did not resolve", e)
			}
		})
	}
}
