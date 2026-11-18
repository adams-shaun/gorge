package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/deck"
)

// synthConflicts are the flags that pick a pool, which -synth already does.
var synthConflicts = []string{"kernel", "set", "decks", "pool", "split", "v2"}

// checkSynthFlags refuses -synth beside a flag that names another pool.
func checkSynthFlags(explicit map[string]bool, with, focusPath string) error {
	if explicit["synth-with"] && (with == "" || focusPath == "") {
		return fmt.Errorf("-synth-with needs a pool list and -focus")
	}
	for _, name := range synthConflicts {
		if explicit[name] {
			return fmt.Errorf("-synth deals its own pool: it cannot be used with -%s", name)
		}
	}
	return nil
}

// needsSynth reports whether a flag that only a -synth run reads was given.
func needsSynth(explicit map[string]bool) bool {
	for _, name := range []string{"synth-decks", "reach", "focus", "focus-per-deck", "only", "synth-with"} {
		if explicit[name] {
			return true
		}
	}
	return false
}

// synthWithExtras is the cards a -synth-with run's union holds from other pools; nil
// without -synth-with. FocusPoolWith keeps them out of the filler.
var synthWithExtras map[string]bool

// synthWithSet is the set of a -synth run with -synth-with: the pool's own table
// widened by the enablers the focus file names that the pool lacks, each taken from
// the first of with that holds it. The union is lowered in this process
// (kgen.SynthUnionTable) because a table is one pool's filters and expressions, so
// two pools' tables cannot be laid side by side. It keeps the main pool's name, so
// the run's verdict and its rows are named as a plain run's are. With no list, or no
// enabler outside the pool, it is the pool's own table.
func synthWithSet(pool string, main cardset.Set, with []string, focusPath, cardsDir string) (cardset.Set, error) {
	b, err := os.ReadFile(focusPath)
	if err != nil {
		return nil, err
	}
	focus, err := oracle.ParseFocus(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", focusPath, err)
	}
	held := func(s cardset.Set, n string) (string, bool) {
		if _, ok := s.Lookup(n); ok {
			return n, true
		}
		f, _, _ := strings.Cut(n, " // ")
		for _, m := range s.Names()[:s.NumCards()] {
			if mf, _, _ := strings.Cut(m, " // "); mf == f {
				return m, true
			}
		}
		return "", false
	}
	var pools []cardset.Set
	for _, w := range with {
		ws, err := cardset.SynthByName(w)
		if err != nil {
			return nil, fmt.Errorf("-synth-with %s: %w", w, err)
		}
		pools = append(pools, ws)
	}
	extras := map[string]bool{}
	for _, fc := range focus {
		for _, n := range fc.With {
			if _, ok := held(main, n); ok {
				continue
			}
			for _, ws := range pools {
				if m, ok := held(ws, n); ok {
					extras[m] = true
					break
				}
			}
		}
	}
	if len(extras) == 0 {
		return main, nil
	}
	names := slices.Clone(main.Names()[:main.NumCards()])
	for n := range extras {
		names = append(names, n)
	}
	slices.Sort(names)
	d, err := kgen.SynthUnionTable(names, cardsDir, pool)
	if err != nil {
		return nil, err
	}
	synthWithExtras = extras
	return cardset.Raw(cardset.NewDataSet(d)), nil
}

// synthSetOf is the set a -synth run plays: the pool's table, or with -synth-with
// the union synthWithSet builds.
func synthSetOf(pool, with, focusPath, cardsDir string) (cardset.Set, error) {
	synthWithExtras = nil // a run in this process starts from its own flags
	set, err := cardset.SynthByName(pool)
	if err != nil || with == "" {
		return set, err
	}
	if cardsDir == "" {
		if cardsDir, err = cardset.CorpusDir(); err != nil {
			return nil, err
		}
	}
	var pools []string
	for _, w := range strings.Split(with, ",") {
		if w = strings.TrimSpace(w); w != "" {
			pools = append(pools, w)
		}
	}
	return synthWithSet(pool, set, pools, focusPath, cardsDir)
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
		pool, files, err = oracle.FocusPoolWith(c.Reg, set, synthWithExtras, focusName, focus, perDeck, seed, nDecks)
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
