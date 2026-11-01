package hbot

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestChooseLandHoldsALandThatWontStay: Arid Archway's enters trigger returns
// a land you control, so with no land out it returns itself. The bot plays it
// only once it controls another.
func TestChooseLandHoldsALandThatWontStay(t *testing.T) {
	deck := []cardset.Entry{{Name: "Arid Archway", Count: 20}, {Name: "Plains", Count: 4}, {Name: "Savannah Lions", Count: 16}}
	set := cardsettest.Set(t, "remparams")
	tbl, da, db, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	f := NewFacts(tbl)
	bt := New(f)
	held, played := 0, 0
	for seed := uint64(1); seed <= 20; seed++ {
		s := dirac.NewConfig(tbl, da, db, seed, dirac.Config{})
		var d dirac.Decision
		for i := 0; i < 3000; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			pick := bt.Pick(s, &d)
			if d.Kind == dirac.DkPriority {
				landOut := false
				for _, o := range s.FeatBattlefield(d.Player, nil) {
					landOut = landOut || tbl.Cards[s.ObjCard(o)].Types&kop.TLand != 0
				}
				for j := 0; j < int(d.N); j++ {
					a := d.Options[j]
					if a.Kind() != dirac.AkPlayLand || f.card(s.ObjCard(dirac.Obj(a.Source()))).EntryReturn == 0 {
						continue
					}
					switch {
					case !landOut && pick == j:
						t.Fatalf("seed %d: played a land that would return itself", seed)
					case !landOut:
						held++
					case pick == j:
						played++
					}
				}
			}
			s.Apply(d.Options[pick])
		}
	}
	if held == 0 || played == 0 {
		t.Fatalf("held %d, played %d: the games reached neither side of the rule", held, played)
	}
}
