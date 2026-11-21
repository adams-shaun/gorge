package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Meld (CR 701.42, 712.4) in lockstep with gorge: Graf Rats + Midnight
// Scavengers (a beginning-of-combat trigger), Hanweir Battlements + Hanweir
// Garrison (an activated meld), Vanille + Fang (an optional paid meld), with
// Murder and Swords to Plowshares so a melded permanent dies and is exiled
// (both cards to the zone, each to its owner's). Pods are 4 seats; the
// 2-seat run is the same decks at two.

// kmeldDecks are the per-seat decks, cycled across the pod's seats.
var kmeldDecks = [][]cardset.Entry{
	{{Name: "Graf Rats", Count: 8}, {Name: "Midnight Scavengers", Count: 8}, {Name: "Murder", Count: 3}, {Name: "Swords to Plowshares", Count: 2}, {Name: "Swamp", Count: 14}, {Name: "Plains", Count: 5}},
	{{Name: "Hanweir Battlements", Count: 5}, {Name: "Hanweir Garrison", Count: 8}, {Name: "Murder", Count: 3}, {Name: "Swords to Plowshares", Count: 2}, {Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 8}, {Name: "Plains", Count: 4}},
	{{Name: "Vanille, Cheerful l'Cie", Count: 6}, {Name: "Fang, Fearless l'Cie", Count: 8}, {Name: "Murder", Count: 3}, {Name: "Swords to Plowshares", Count: 2}, {Name: "Forest", Count: 9}, {Name: "Swamp", Count: 9}, {Name: "Plains", Count: 3}},
}

// runMeldPods plays seats seats over seeds seeds, gorge leading, and returns
// the games, the divergences and the stalls.
func runMeldPods(t *testing.T, seats, seeds int, report bool) (games, divs, stalls int) {
	t.Helper()
	c, _ := testCorpus(t)
	set, err := cardset.RawByName("meld")
	if err != nil {
		t.Fatal(err)
	}
	md := make([][]cardset.Entry, seats)
	names := make([]string, seats)
	gd := make([][]*cards.Card, seats)
	for s := 0; s < seats; s++ {
		md[s], names[s] = kmeldDecks[s%len(kmeldDecks)], fmt.Sprint("kmeld-", s)
		for _, e := range md[s] {
			cc, ok := c.Reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], cc)
			}
		}
	}
	tbl, _, err := set.MatchupN(md)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchupN(c.Reg, names, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 60})
		games++
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			if report {
				t.Logf("seed %d: kernel stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if report {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		CloseSide(k)
	}
	return games, divs, stalls
}

func meldLockstep(t *testing.T, seats, seeds int) {
	var st dirac.MeldStats
	dirac.DebugMeldStats = &st
	defer func() { dirac.DebugMeldStats = nil }()
	games, divs, stalls := runMeldPods(t, seats, seeds, true)
	t.Logf("%d seats: %d games, %d melds, %d splits, %d stay-exiled", seats, games, st.Melds, st.Splits, st.Stays)
	if st.Melds == 0 || st.Splits == 0 {
		t.Fatalf("the pods never exercised meld: %d melds, %d splits", st.Melds, st.Splits)
	}
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d/%d games diverged, %d stalls", divs, games, stalls)
	}
}

// TestMeldLockstep2 and TestMeldLockstep4: every game agrees decision by
// decision with gorge, and a meld and a split really happen.
func TestMeldLockstep2(t *testing.T) { meldLockstep(t, 2, 40) }
func TestMeldLockstep4(t *testing.T) { meldLockstep(t, 4, 24) }

// TestMeldMutation: a melded permanent that does not split on leaving
// (DebugMeldKeep) must diverge from gorge on some seed.
func TestMeldMutation(t *testing.T) {
	dirac.DebugMeldKeep = true
	defer func() { dirac.DebugMeldKeep = false }()
	var st dirac.MeldStats
	dirac.DebugMeldStats = &st
	defer func() { dirac.DebugMeldStats = nil }()
	_, divs, _ := runMeldPods(t, 4, 24, false)
	if divs == 0 {
		t.Fatalf("keeping the partner parked never diverged (%d melds)", st.Melds)
	}
}
