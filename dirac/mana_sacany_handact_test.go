package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSacAnyRepeatFundsCycling: Diamond Kaleidoscope's "Sacrifice a Prism
// token: Add one mana of any color" is repeatable, so two Prism tokens pay
// Titanoth Rex's cycling {1}{G} from the hand (gorge's autoPayOffer; CR 602.2b,
// CR 605.3a; tail-12-c000 pair 93, decision 531). One token leaves it unoffered.
func TestSacAnyRepeatFundsCycling(t *testing.T) {
	for _, c := range []struct {
		name   string
		prisms int
		want   bool
	}{
		{"one Prism cannot fund the cycling", 1, false},
		{"two Prism sacrifices pay the cycling", 2, true},
	} {
		for _, cfg := range ksacCfgs {
			t.Run(c.name, func(t *testing.T) {
				sc, id := synthScenCfg(t, "tail-12", cfg, "Titanoth Rex", "Diamond Kaleidoscope")
				prism := CardID(0xffff)
				for cid := range sc.k.t.Cards {
					if sc.k.t.Cards[cid].Name == "c_0_1_a_prism" {
						prism = CardID(cid)
					}
				}
				if prism == 0xffff {
					t.Fatal("the matchup table holds no prism token")
				}
				rex := sc.hand(id["Titanoth Rex"], 0)
				sc.perm(id["Diamond Kaleidoscope"], 0)
				for i := 0; i < c.prisms; i++ {
					w := sc.perm(prism, 0)
					sc.s.objs[w].of |= ofToken
				}
				// precondition: the ability index of the hand-zone cycling
				ab := -1
				for i := range sc.k.t.Cards[id["Titanoth Rex"]].Abilities {
					if a := &sc.k.t.Cards[id["Titanoth Rex"]].Abilities[i]; a.Kind == kop.AbActivated && a.ActZone == kop.ZoneHand {
						ab = i
					}
				}
				if ab < 0 {
					t.Fatal("Titanoth Rex has no hand-zone activated ability")
				}
				if got := offersAb(sc.s, rex, uint8(ab)); got != c.want {
					t.Errorf("cycling offered %v, want %v", got, c.want)
				}
			})
		}
	}
}

// synthScenCfg is synthScen under cfg.
func synthScenCfg(t *testing.T, pool string, cfg Config, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Synth(t, pool)
	var deck []cardset.Entry
	for _, n := range names {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTable(t, tbl, cfg), id
}
