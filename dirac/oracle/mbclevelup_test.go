package oracle

import (
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestMBCLevelupLowers plays the Level up creatures kgen lowers (CR 702.87:
// the sorcery-speed "put a level counter" ability, and the level bands'
// Self-counter statics, one comparison per kernel filter clause) in lockstep
// against gorge. Each card sits in its MBC half pool beside basic lands of
// its colour; the game must reach the card's level-up activation so the
// bands (power/toughness, keywords, the granted tap abilities) are compared.
func TestMBCLevelupLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	cases := []struct{ card, pool, land string }{
		{"Enclave Cryptologist", "dr-mbc-aa", "Island"},
		{"Knight of Cliffhaven", "dr-mbc-ba", "Plains"},
		{"Joraga Treespeaker", "dr-mbc-ab", "Forest"},
		{"Nirkana Cutthroat", "dr-mbc-ba", "Swamp"},
		{"Hada Spy Patrol", "dr-mbc-bb", "Island"},
	}
	for _, c := range cases {
		t.Run(c.card, func(t *testing.T) {
			set := cardsettest.Synth(t, c.pool)
			if _, ok := set.Lookup(c.card); !ok {
				t.Fatalf("%s is not lowered into %s", c.card, c.pool)
			}
			a := []cardset.Entry{{Name: c.card, Count: 16}, {Name: c.land, Count: 24}}
			b := []cardset.Entry{{Name: "Plains", Count: 40}}
			var mu sync.Mutex
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
						mu.Lock()
						levelled++
						mu.Unlock()
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
