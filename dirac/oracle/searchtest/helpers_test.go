// Package searchtest holds the lockstep tests that need dirac/search or
// dirac/neteval, which package oracle may not import: the search's
// declarer against gorge's heuristic bot, the network evaluator against
// gorge's gonet on the same positions, and dirac/feat's parts encoding
// against the materialized one.
package searchtest

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/deck"
)

var (
	corpusOnce sync.Once
	corpusVal  *oracle.Corpus
	poolVal    []oracle.PoolDeck
	corpusErr  string
)

// testCorpus opens the card corpus and a deck pool once: the FDN pool, else decks
// dealt from its table. It skips t when this box lacks the corpus.
func testCorpus(t testing.TB) (*oracle.Corpus, []oracle.PoolDeck) {
	t.Helper()
	corpusOnce.Do(func() {
		dir, err := cardset.CorpusDir()
		if err != nil {
			corpusErr = err.Error()
			return
		}
		if _, err := os.Stat(filepath.Join(dir, "cardsfolder")); err != nil {
			corpusErr = "no gorge corpus at " + dir
			return
		}
		in := oracle.PoolInputs("fdn")
		c, err := oracle.OpenCorpus(dir, filepath.Join(in, "decks"))
		if err != nil {
			corpusErr = err.Error()
			return
		}
		pool, err := oracle.ReadPool(filepath.Join(in, "pool.tsv"), "all")
		if err != nil {
			// no deck pool: a clone without one plays decks dealt from the same table
			var files map[string]deck.File
			pool, files, err = oracle.SynthPool(c.Reg, rawSet("fdn"), 1, oracle.SynthDecks)
			if err != nil {
				corpusErr = err.Error()
				return
			}
			c.AddDecks(files)
		}
		corpusVal, poolVal = c, pool
	})
	if corpusVal == nil {
		t.Skip(corpusErr)
	}
	return corpusVal, poolVal
}

// rawSetT is rawSet for a test: it skips when no card corpus is built.
func rawSetT(tb testing.TB, name string) cardset.Set {
	tb.Helper()
	s, err := cardset.RawByName(name)
	if errors.Is(err, cardset.ErrNoCorpus) {
		tb.Skipf("set %s: %v", name, err)
	}
	if err != nil {
		tb.Fatalf("set %s: %v", name, err)
	}
	return s
}

// rawSet is the named set's raw table, every lowered card admitted.
func rawSet(name string) cardset.Set {
	s, err := cardset.RawByName(name)
	if err != nil {
		panic(err)
	}
	return s
}
