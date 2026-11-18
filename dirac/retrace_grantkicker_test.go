package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRetraceGrantKickerNotBarred: a kicked cast is a hand-only offer (gorge
// legal_walk_hand.go offers the kicked mode from the hand and the kicker cost
// folds only in cast_begin.go's castModeKicked), so a Kicker card's retrace
// cast is the plain one in both engines: the printed base plus the land
// discard, the kicker unpaid, and neither engine offers a kicked retrace
// cast. Six's granted retrace on a Kicker card (mana kicker, or a non-mana
// KickerCost) is therefore offered, not withheld as a retracegrant stall.
func TestRetraceGrantKickerNotBarred(t *testing.T) {
	deck := []cardset.Entry{{Name: "Six", Count: 1}, {Name: "Hill Giant", Count: 1}, {Name: "Forest", Count: 10}}
	base, da, _, err := cardsettest.Set(t, "offzonegrants").Matchup(deck, deck)
	if err != nil {
		t.Skip(err)
	}
	cases := []struct {
		name string
		mut  func(cd *kop.CardDef)
	}{
		{"kicker_mana", func(cd *kop.CardDef) {
			cd.Keywords |= kop.KwKicker // Phyrexian Missionary's shape: Kicker {1}{B}
			cd.Kicker = kop.ManaCost{Generic: 1, Pips: [kop.NumManaKinds]uint8{2: 1}}
		}},
		{"kicker_cost", func(cd *kop.CardDef) {
			// Primal Growth's shape: Kicker Sac<1/Creature>, the non-mana part
			// whose pick a plain cast never owes (sfKicked is unset)
			cd.Keywords |= kop.KwKicker
			cd.KickerCost = kop.Cost{Flags: kop.CostSac, N: 1}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tbl := *base // the card is mutated on a private copy of the table
			tbl.Cards = slices.Clone(base.Cards)
			id := map[string]CardID{}
			for _, c := range da {
				id[tbl.Cards[c].Name] = c
			}
			giant := id["Hill Giant"]
			tbl.Cards[giant].Cost = kop.ManaCost{Generic: 3, Pips: [kop.NumManaKinds]uint8{kop.G: 1}} // green: the fixture's only source colour
			tc.mut(&tbl.Cards[giant])
			sc := newScenTableLib(t, &tbl, id["Forest"], Config{NoElide: true})
			sc.perm(id["Six"], 0)
			for i := 0; i < 4; i++ {
				sc.perm(id["Forest"], 0) // the printed {3}{G} (Hill Giant)
			}
			sc.hand(id["Forest"], 0) // the retrace cast's discardable land
			sc.s.refreshTy(sc.k)
			grave := sc.grave(giant, 0, 1)[0]
			if !sc.s.retraceLive(sc.k, grave) {
				t.Fatalf("precondition: Six must grant the graveyard Hill Giant retrace")
			}
			if retraceGrantBarred(sc.k, giant) {
				t.Fatalf("retraceGrantBarred: a %s card's granted retrace is withheld", tc.name)
			}
			var d Decision
			var av avail
			sc.s.offerRetraceCasts(sc.k, &d, 0, true, &av)
			if r := sc.s.StallReason(); r == StallRetraceGrant {
				t.Fatalf("stall %q on a %s card's granted retrace", StallName(r), tc.name)
			}
			found := false
			for i := 0; i < int(d.N); i++ {
				a := d.Options[i]
				if a.Kind() == AkCast && a.Source() == TCode(grave) && a.Target() == TCode(retraceTgt) && a.Mode() == 2 {
					found = true
				}
			}
			if !found {
				t.Fatalf("no plain retrace cast of the %s card was offered (%d options)", tc.name, d.N)
			}
		})
	}
}
