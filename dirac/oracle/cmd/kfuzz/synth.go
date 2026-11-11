package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/deck"
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

// focusName is the focus pool a -focus run deals, which names its verdict; "" otherwise.
var focusName string

// reachFunc reports a finished pool run: its pairs, their outcomes and the coverage
// of its clean games.
type reachFunc func(ps []oracle.Pairing, out []played, cov *oracle.Coverage) error

// dealSynth deals set's pool into the corpus's memory and returns the pool and the
// report that goes with it. The deal takes -seed, so -seed reproduces the decks as
// well as the pairs, and a rolling seed is a new deal.
//
// With focusPath it deals the focus pool that file lists (oracle.FocusPool) instead,
// named after the file, and the verdict and reach carry that name.
func dealSynth(c *oracle.Corpus, set cardset.Set, seed uint64, nDecks int, reachPath, focusPath string, perDeck int) ([]oracle.PoolDeck, reachFunc, error) {
	var pool []oracle.PoolDeck
	var files map[string]deck.File
	var err error
	if focusPath == "" {
		pool, files, err = oracle.SynthPool(c.Reg, set, seed, nDecks)
	} else {
		b, rerr := os.ReadFile(focusPath)
		if rerr != nil {
			return nil, nil, rerr
		}
		focus, perr := oracle.ParseFocus(b)
		if perr != nil {
			return nil, nil, fmt.Errorf("%s: %w", focusPath, perr)
		}
		focusName = strings.TrimSuffix(filepath.Base(focusPath), ".txt")
		pool, files, err = oracle.FocusPool(c.Reg, set, focusName, focus, perDeck, seed, nDecks)
	}
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
		if focusName != "" {
			r.Pool = focusName
		}
		fmt.Println(r.Line())
		if reachPath == "" {
			return nil
		}
		return os.WriteFile(reachPath, r.TSV(), 0o644)
	}, nil
}
