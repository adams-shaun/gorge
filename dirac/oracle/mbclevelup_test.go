package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestMBCLevelupLowers plays the Level up creatures kgen lowers (CR 702.87:
// the sorcery-speed "put a level counter" ability, and the level bands'
// Self-counter statics, one comparison per kernel filter clause) in lockstep
// against gorge. Each card is played from whichever dr-mbc-* synthetic pool
// holds it (the hash cut moves as other MBC cards lower, so the pool is
// searched, not named), beside basic lands of its colour; the game must reach the card's level-up activation so the
// bands (power/toughness, keywords, the granted tap abilities) are compared.
func TestMBCLevelupLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	cases := []struct{ card, land string }{
		{"Enclave Cryptologist", "Island"},
		{"Knight of Cliffhaven", "Plains"},
		{"Joraga Treespeaker", "Forest"},
		{"Nirkana Cutthroat", "Swamp"},
		{"Hada Spy Patrol", "Island"},
	}
	var mbc []string
	for _, n := range cardsettest.SynthNames(t) {
		if strings.HasPrefix(n, "dr-mbc-") {
			mbc = append(mbc, n)
		}
	}
	for _, c := range cases {
		t.Run(c.card, func(t *testing.T) {
			set, found := mbcPoolHolding(t, mbc, c.card)
			if !found {
				t.Fatalf("%s is not lowered into any dr-mbc-* pool %v", c.card, mbc)
			}
			a := []cardset.Entry{{Name: c.card, Count: 16}, {Name: c.land, Count: 24}}
			b := []cardset.Entry{{Name: "Plains", Count: 40}}
			levelled := 0
			reg := synthReg(t)
			var cs [2][]*cards.Card
			for s, d := range [2][]cardset.Entry{a, b} {
				for _, e := range d {
					cd, ok := reg.Lookup(e.Name)
					if !ok {
						t.Fatalf("%s not in the corpus", e.Name)
					}
					for i := 0; i < e.Count; i++ {
						cs[s] = append(cs[s], cd)
					}
				}
			}
			tbl, _, _, err := set.Matchup(a, b)
			if err != nil {
				t.Fatal(err)
			}
			for seed := uint64(1); seed <= 12; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"fixture-a", "fixture-b"}, cs, 0)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
				CloseSide(k)
				if d := res.Div; d != nil {
					t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
				for _, a := range res.Actions {
					if strings.HasPrefix(a.String(), "activate(") && firstObjectCard(m, a.String()) == c.card {
						levelled++
					}
				}
			}
			t.Logf("%s: %d level-up activations over 12 games", c.card, levelled)
			if levelled == 0 {
				t.Fatalf("%s: no game activated Level up", c.card)
			}
		})
	}
}

// mbcPoolHolding is the first of the dr-mbc-* synthetic pools whose lowered
// set holds card.
func mbcPoolHolding(t *testing.T, pools []string, card string) (cardset.Set, bool) {
	t.Helper()
	for _, n := range pools {
		set := cardsettest.Synth(t, n)
		if _, ok := set.Lookup(card); ok {
			return set, true
		}
	}
	return nil, false
}
