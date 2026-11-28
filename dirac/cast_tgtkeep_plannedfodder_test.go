package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A planned payment may sacrifice the spell's announced target (CR 601.2c-h)
// only through a source gorge's payment plan uses: planOtherSacSource, a plain
// creature victim. Diamond Kaleidoscope's "Sacrifice a Prism token: Add one
// mana of any color" is no such source, so a cast only it funds by taking the
// sole Prism target is no planned cast.
func TestPlannedCastKeepsSoleTargetFromUnplannedSacSource(t *testing.T) {
	for _, spell := range []string{"Sudden Spinnerets", "Veteran's Reflexes"} {
		for _, cfg := range ksacCfgs {
			t.Run(spell, func(t *testing.T) {
				// second: a second creature stays a target after the payment
				planned := func(second bool) bool {
					sc, id := synthScenCfg(t, "tail-12", cfg, spell, "Diamond Kaleidoscope", "Titanoth Rex")
					prism := CardID(0xffff)
					for cid := range sc.k.t.Cards {
						if sc.k.t.Cards[cid].Name == "c_0_1_a_prism" {
							prism = CardID(cid)
						}
					}
					if prism == 0xffff {
						t.Fatal("precondition: the matchup table holds no prism token")
					}
					o := sc.hand(id[spell], 0)
					kal := sc.perm(id["Diamond Kaleidoscope"], 0)
					fodder := sc.perm(prism, 0)
					sc.s.objs[fodder].of |= ofToken
					if second {
						sc.perm(id["Titanoth Rex"], 0)
					}
					if sc.s.planOtherSacSource(sc.k, sc.slot(kal)) {
						t.Fatal("precondition: Diamond Kaleidoscope reads as a source gorge's plan uses")
					}
					sp := tspec(sc.k, id[spell], spellAb(sc.k, id[spell]), 0)
					if sp == nil {
						t.Fatal("precondition: the spell announces no target")
					}
					var buf [MaxOptions]TCode
					want := 1
					if second {
						want = 2
					}
					if n := sc.s.targetCands(sc.k, sp, 0, NoSlot, o, &buf); n != want {
						t.Fatalf("precondition: %d target candidates, want %d", n, want)
					}
					cost := sc.k.cards[id[spell]].cost
					if cost == (kop.ManaCost{}) || sc.s.plainCensusPays(sc.k, 0, o, &cost) {
						t.Fatalf("precondition: the cost %+v needs no gap source", cost)
					}
					return sc.s.FeatPlannedCast(0, o, 0)
				}
				if !planned(true) {
					t.Fatal("precondition: the Prism sacrifice beside a second creature is no planned cast")
				}
				if planned(false) {
					t.Error("FeatPlannedCast = true when only an unplanned sacrifice of the sole target pays")
				}
			})
		}
	}
}
