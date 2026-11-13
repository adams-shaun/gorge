package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestGoblinTraprunnerLockstep plays Goblin Traprunner mirrors (no synthetic
// pool holds it: the table is built from the decks, printedDelveSet) in
// lockstep and requires no divergence and at least one three-coin batch: a
// FlipCoin Amount$ 3 that lowers to Repeat over a batch flip, the
// per-flip Wins read and the closing Mark (dirac/kgen lower_dice.go).
func TestGoblinTraprunnerLockstep(t *testing.T) {
	t.Parallel()
	const card = "Goblin Traprunner"
	reg := synthReg(t)
	deck := []cardset.Entry{{Name: card, Count: 14}, {Name: "Mountain", Count: 26}}
	decks := [2][]cardset.Entry{deck, deck}
	set := printedDelveSet(t, [2][]string{{card, "Mountain"}, {card, "Mountain"}})
	var cs [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	batches := 0
	for seed := uint64(1); seed <= 60; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"traprunner-a", "traprunner-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		saw := false
		rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40}
		rc.Check = func(ref *GorgeSide, ker Side) *Divergence {
			// each Traprunner attack flips exactly three coins, so a
			// player with three flips this turn finished a batch
			for _, p := range ref.E.G.Players {
				if p.CoinFlipsThisTurn >= 3 {
					saw = true
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
		}
		if saw {
			batches++
		}
	}
	if batches == 0 {
		t.Fatal("no seed reached a three-coin batch")
	}
	t.Logf("%d of 60 seeds reached a three-coin batch", batches)
}
