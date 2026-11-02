package main

import (
	"fmt"
	"os"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
)

// synthConflicts are the flags that pick a pool, which -synth already does.
var synthConflicts = []string{"kernel", "set", "decks", "pool", "split", "v2"}

// checkSynthFlags refuses -synth beside a flag that names another pool.
func checkSynthFlags(explicit map[string]bool) error {
	for _, name := range synthConflicts {
		if explicit[name] {
			return fmt.Errorf("-synth deals its own pool: it cannot be used with -%s", name)
		}
	}
	return nil
}

// reachFunc reports a finished pool run: its pairs, their outcomes and the coverage
// of its clean games.
type reachFunc func(ps []oracle.Pairing, out []played, cov *oracle.Coverage) error

// dealSynth deals set's pool into the corpus's memory and returns the pool and the
// report that goes with it. The deal takes -seed, so -seed reproduces the decks as
// well as the pairs, and a rolling seed is a new deal.
func dealSynth(c *oracle.Corpus, set cardset.Set, seed uint64, nDecks int, reachPath string) ([]oracle.PoolDeck, reachFunc, error) {
	pool, files, err := oracle.SynthPool(c.Reg, set, seed, nDecks)
	if err != nil {
		return nil, nil, err
	}
	c.AddDecks(files)
	return pool, func(ps []oracle.Pairing, out []played, cov *oracle.Coverage) error {
		var started [][2]string
		for i, p := range ps {
			if out[i].skip == nil && out[i].err == nil {
				started = append(started, [2]string{p.A, p.B})
			}
		}
		r := oracle.SynthReach(set, pool, files, started, cov)
		fmt.Println(r.Line())
		if reachPath == "" {
			return nil
		}
		return os.WriteFile(reachPath, r.TSV(), 0o644)
	}, nil
}
