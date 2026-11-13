package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestJoinForcesLockstep: the join-forces cards (each player may pay any
// amount of mana; X is the total paid) play identically in gorge and dirac:
// 24 games per card, 8 copies and 32 basics a seat, casting or activating
// whenever it can. A card that is never played proves nothing, so a play
// count of zero fails.
func TestJoinForcesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	for _, c := range []struct{ name, basic string }{
		{"Minds Aglow", "Island"},
		{"Alliance of Arms", "Plains"},
		{"Shared Trauma", "Swamp"},
		{"Mana-Charged Dragon", "Mountain"},
	} {
		t.Run(c.name, func(t *testing.T) {
			deck := []cardset.Entry{{Name: c.name, Count: 8}, {Name: c.basic, Count: 32}}
			var cs [2][]*cards.Card
			for s := range cs {
				for _, e := range deck {
					card, ok := reg.Lookup(e.Name)
					if !ok {
						t.Fatalf("%s not in the corpus", e.Name)
					}
					for i := 0; i < e.Count; i++ {
						cs[s] = append(cs[s], card)
					}
				}
			}
			set, tbl := joinForcesPool(t, deck)
			played := map[string]int{}
			for seed := uint64(1); seed <= 24; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"jf-a", "jf-b"}, cs, 0)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				res := Lockstep(m, seed, k, castActivatePlayedPolicy(seed, played), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
				CloseSide(k)
				if d := res.Div; d != nil {
					t.Logf("last actions %v", actionStrings(res.Actions[max(0, d.At-25):d.At]))
					t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
				}
			}
			if played[c.name] == 0 {
				t.Fatalf("%s was never played in 24 games; raise its copies", c.name)
			}
			t.Logf("%s played %d times", c.name, played[c.name])
		})
	}
}

// joinForcesPool is the synthetic pool holding every card of deck, and its
// matchup table. These cards are outside the Standard lists, so they sit in a
// tail-NN bucket that moves whenever the lowered set grows: the pool is found
// by name, never fixed.
func joinForcesPool(t *testing.T, deck []cardset.Entry) (cardset.Set, *kop.Table) {
	t.Helper()
	for _, name := range cardsettest.SynthNames(t) {
		if !strings.HasPrefix(name, "tail-") {
			continue
		}
		set := cardsettest.Synth(t, name)
		if tbl, _, _, err := set.Matchup(deck, deck); err == nil {
			return set, tbl
		}
	}
	t.Fatalf("no synthetic tail pool holds %v", deck)
	return nil, nil
}
