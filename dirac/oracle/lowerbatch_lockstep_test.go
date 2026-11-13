package oracle

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// fixtureLockstep plays seeds games of a fixture pool of a synthetic set's
// cards in lockstep, gorge leading, alternating the two option surfaces, and
// fails on the first divergence.
func fixtureLockstep(t *testing.T, setName string, decks [2][]cardset.Entry, seeds int) {
	t.Helper()
	fixtureLockstepSeeds(t, setName, decks, 1, seeds)
}

// fixtureLockstepSeeds is fixtureLockstep over seeds first to last.
func fixtureLockstepSeeds(t *testing.T, setName string, decks [2][]cardset.Entry, first, last int) {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, setName)
	var cs [2][]*cards.Card
	for s, d := range decks {
		n := 0
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d has %d cards", s, n)
		}
	}
	seedChunks(t, uint64(first), uint64(last), fixtureChunk, func(t *testing.T, lo, hi uint64, _ *sync.Mutex) {
		tbl, _, _, err := set.Matchup(decks[0], decks[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := lo; seed <= hi; seed++ {
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
				t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
			}
		}
	})
}

// fixtureChunk is the seeds one parallel subtest of fixtureLockstep plays
// (seedChunks).
const fixtureChunk = 15

// Dispatch's Metalcraft link exiles the tapped creature only with three or
// more artifacts (CR 207.2c); the deck holds artifacts to reach the threshold
// and creatures to target.
func TestMetalcraftConditionLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{{Name: "Dispatch", Count: 10}, {Name: "Mind Stone", Count: 5}, {Name: "Howling Mine", Count: 3}, {Name: "Plains", Count: 22}}
	b := []cardset.Entry{{Name: "Exocomp", Count: 10}, {Name: "Gorn Captain", Count: 6}, {Name: "Ceti Eel", Count: 4}, {Name: "Mind Stone", Count: 4}, {Name: "Island", Count: 16}}
	fixtureLockstep(t, "std-trc", [2][]cardset.Entry{a, b}, 300)
}

// Unlucky Cabbage Merchant puts itself on the bottom of its library and
// shuffles when a Food is sacrificed (LibraryPosition$ -1 with Shuffle$ True);
// the Food tokens it makes pay for themselves from the lands.
func TestBottomShuffleLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{{Name: "Unlucky Cabbage Merchant", Count: 14}, {Name: "Forest", Count: 26}}
	b := []cardset.Entry{{Name: "Unlucky Cabbage Merchant", Count: 6}, {Name: "Forest", Count: 20}, {Name: "Mountain", Count: 14}}
	fixtureLockstep(t, "std-tla", [2][]cardset.Entry{a, b}, 300)
}
