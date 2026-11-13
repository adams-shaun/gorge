package hbot

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPotentialPricesEitherOrPart: gorge's potential walk offers a cast with an
// either-or additional cost only when some part is payable with the mana cost
// (legal_walk_hand.go altAddOfferCastable), so Eaten Alive with no creature to
// sacrifice is in decision.PotentialCasts only beside five black sources
// ({B} plus {3}{B}). Priced at its printed {B} it was the tap gate's target
// over two lands, and hbot floated mana by hand toward it where gorge cast a
// planned Stab (FDN pod: Dismal Backwater and an Island).
func TestPotentialPricesEitherOrPart(t *testing.T) {
	deck := []cardset.Entry{{Name: "Eaten Alive", Count: 16}, {Name: "Swamp", Count: 24}}
	set := cardsettest.Set(t, "delaydies")
	tbl, da, db, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	bt := New(NewFacts(tbl))
	var short, enough int
	for seed := uint64(1); seed <= 4; seed++ {
		s := dirac.NewConfig(tbl, da, db, seed, dirac.Config{GorgeAutopaySurface: true})
		var d dirac.Decision
		for i := 0; i < 3000; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if d.Kind == dirac.DkPriority {
				bt.b.fillPriority(s, bt.f, d.Player, &d)
				var black int32
				for j := 0; j < bt.b.nCard; j++ {
					if x := &bt.b.card[j]; x.own && x.onBF && !x.tapped {
						black++
					}
				}
				black += bt.b.pool[2]
				for j := 0; j < bt.b.nCard; j++ {
					c := &bt.b.card[j]
					if !c.own || s.ObjZone(c.obj) != kop.ZoneHand || tbl.Cards[s.ObjCard(c.obj)].Name != "Eaten Alive" {
						continue
					}
					want := black >= 5
					if want {
						enough++
					} else {
						short++
					}
					if got := bt.potential(c); got != want {
						t.Fatalf("seed %d step %d: potential(Eaten Alive) = %v with %d black mana and no creature, want %v", seed, s.StepNow(), got, black, want)
					}
				}
			}
			s.Apply(d.Options[bt.Pick(s, &d)])
		}
	}
	if short == 0 || enough == 0 {
		t.Fatalf("census short %d enough %d: both sides of the five-source line must be seen", short, enough)
	}
}
