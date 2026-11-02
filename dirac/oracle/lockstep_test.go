package oracle

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

var (
	corpusOnce sync.Once
	corpusVal  *Corpus
	poolVal    []PoolDeck
	corpusErr  string
	// corpusRealPool: poolVal is the FDN pool, not decks dealt in its place
	corpusRealPool bool
)

// testCorpus opens the gorge corpus and a deck pool: KORACLE_DECKS, else
// PoolInputs("fdn"), else decks dealt from the fdn table. A box without the corpus
// skips the corpus tests and says so.
func testCorpus(t testing.TB) (*Corpus, []PoolDeck) {
	t.Helper()
	corpusOnce.Do(func() {
		cardsDir, err := cardset.CorpusDir()
		if err != nil {
			corpusErr = err.Error()
			return
		}
		if _, err := os.Stat(filepath.Join(cardsDir, "cardsfolder")); err != nil {
			corpusErr = "no gorge corpus at " + cardsDir
			return
		}
		in := os.Getenv("KORACLE_DECKS")
		if in == "" {
			in = PoolInputs("fdn")
		}
		c, err := OpenCorpus(cardsDir, filepath.Join(in, "decks"))
		if err != nil {
			corpusErr = err.Error()
			return
		}
		pool, err := ReadPool(filepath.Join(in, "pool.tsv"), "all")
		if err != nil {
			// no deck pool: a clone without one plays synthetic decks of the same set
			pool, err = addSynthFDNDecks(c)
		} else {
			corpusRealPool = true
		}
		if err != nil {
			corpusErr = err.Error()
			return
		}
		corpusVal, poolVal = c, pool
	})
	if corpusVal == nil {
		t.Skip(corpusErr)
	}
	return corpusVal, poolVal
}

// testFDNPool is testCorpus for a test pinned to the real FDN pool's decks: named
// decks, recorded pairs, or a rate only that pool's size holds.
func testFDNPool(t testing.TB) (*Corpus, []PoolDeck) {
	t.Helper()
	c, pool := testCorpus(t)
	if !corpusRealPool {
		t.Skip("needs the FDN deck pool (set GORGE_POOLS or KORACLE_DECKS)")
	}
	return c, pool
}

func addSynthFDNDecks(c *Corpus) ([]PoolDeck, error) {
	set, err := cardset.RawByName("fdn")
	if err != nil {
		return nil, err
	}
	pool, files, err := SynthPool(c.Reg, set, 1, SynthDecks)
	if err != nil {
		return nil, err
	}
	c.AddDecks(files)
	return pool, nil
}

// TestLockstepSmoke plays random FDN pairings gorge-vs-gorge (GorgeKernel)
// under the random policy, with and without pass elision and with a
// mulligan round, and requires zero divergences.
func TestLockstepSmoke(t *testing.T) {
	t.Parallel()

	c, pool := testCorpus(t)
	pairs := Pairings(pool, 6, 20261017)
	for i, p := range pairs {
		cfg := RunConfig{Config: Config{PassElision: i%2 == 0}}
		m, err := c.Matchup(p.A, p.B, i%3)
		if err != nil {
			t.Fatal(err)
		}
		k, err := GorgeKernel(cfg.Config)(m, p.Seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), cfg)
		if res.Div != nil {
			t.Fatalf("pair %d (%s vs %s seed %d): %v\n%v\nref %v\nker %v", i, p.A, p.B, p.Seed, res.Div, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions)
		}
		if !res.Over && res.Stall == "" {
			t.Fatalf("pair %d: game neither over nor stalled", i)
		}
		t.Logf("pair %d %s vs %s: %d decisions, turn %d, winner %d, stall %q, stats %+v", i, p.A, p.B, res.Decisions, res.Turns, res.Winner, res.Stall, res.Stats)
	}
}
