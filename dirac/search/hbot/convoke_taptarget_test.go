package hbot

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapTargetConvokeShapeCast: gorge's planner declines a convoke cast for
// its shape ("shape:contribution"), so The Wandering Rescuer (flash, convoke)
// sits in decision.PotentialShapeCasts and is the tap gate's target whenever
// the untapped sources could pay it, though the autopay surface offers no cast
// of it. The card-only refusal (planNeverPricesOf) misses convoke; read alone
// it dropped the Rescuer from the census, so wantsManual fell to a lesser
// planned cast (std-dsk: Twist Reality cast where gorge passes).
func TestTapTargetConvokeShapeCast(t *testing.T) {
	deck := []cardset.Entry{{Name: "The Wandering Rescuer", Count: 16}, {Name: "Plains", Count: 24}}
	set := cardsettest.Synth(t, "std-dsk")
	tbl, da, db, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	bt := New(NewFacts(tbl))
	hits := 0
	for seed := uint64(1); seed <= 8 && hits == 0; seed++ {
		s := dirac.NewConfig(tbl, da, db, seed, dirac.Config{GorgeAutopaySurface: true})
		var d dirac.Decision
		for i := 0; i < 3000; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if d.Kind == dirac.DkPriority {
				bt.b.fillPriority(s, bt.f, d.Player, &d)
				for j := 0; j < bt.b.nCard; j++ {
					c := &bt.b.card[j]
					o := c.obj
					if !c.own || s.ObjZone(o) != kop.ZoneHand || tbl.Cards[s.ObjCard(o)].Name != "The Wandering Rescuer" || offersCast(&d, o) {
						continue
					}
					if !c.castable || !bt.potential(c) {
						continue // the sources cannot pay it: no census entry either side
					}
					if !c.instant {
						t.Fatalf("seed %d: the Rescuer (obj %d) is not an instant-speed card", seed, o)
					}
					hits++
					if !bt.tapTarget(&d, c) {
						t.Fatalf("seed %d step %d: tapTarget refuses the unoffered convoke Rescuer (obj %d) the sources pay", seed, s.StepNow(), o)
					}
				}
			}
			s.Apply(d.Options[bt.Pick(s, &d)])
		}
	}
	if hits == 0 {
		t.Fatal("no priority decision held an unoffered Rescuer the sources pay")
	}
}

// offersCast reports whether d offers a cast of o.
func offersCast(d *dirac.Decision, o dirac.Obj) bool {
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == dirac.AkCast && dirac.Obj(a.Source()) == o {
			return true
		}
	}
	return false
}
