package searchprobe

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

// LoadBeliefDir builds a closed-list belief from dir: the decks pool (a
// pool.tsv of "deck<TAB>split<TAB>..." with a header row) names as its train
// split, or every deck file in dir when pool is "". A deck the corpus cannot
// resolve is skipped. Errors name dir or pool; callers wrap them with the
// flag that supplied the path. It is the one reader behind gorged's
// -<policy>-belief flags.
func LoadBeliefDir(reg *cards.Registry, dir, pool string) (*Belief, error) {
	names, err := beliefDeckNames(dir, pool)
	if err != nil {
		return nil, err
	}
	var decks [][]*cards.Card
	for _, n := range names {
		_, d, err := deck.Load(reg, filepath.Join(dir, n))
		if err != nil {
			continue // a deck the corpus cannot resolve is no evidence either way
		}
		decks = append(decks, d)
	}
	b, err := NewBelief(decks)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", dir, err)
	}
	return b, nil
}

// beliefDeckNames lists the deck files LoadBeliefDir reads, sorted.
func beliefDeckNames(dir, pool string) ([]string, error) {
	var names []string
	if pool != "" {
		raw, err := os.ReadFile(pool)
		if err != nil {
			return nil, &PoolError{Err: err}
		}
		for i, line := range strings.Split(string(raw), "\n") {
			v := strings.Split(line, "\t")
			if i == 0 || len(v) < 2 || v[1] != "train" {
				continue
			}
			names = append(names, v[0]+".json")
		}
	} else {
		ents, err := os.ReadDir(dir)
		if err != nil {
			return nil, &DirError{Err: err}
		}
		for _, e := range ents {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				names = append(names, e.Name())
			}
		}
	}
	sort.Strings(names)
	return names, nil
}

// PoolError is LoadBeliefDir's failure to read the pool file.
type PoolError struct{ Err error }

func (e *PoolError) Error() string { return e.Err.Error() }
func (e *PoolError) Unwrap() error { return e.Err }

// DirError is LoadBeliefDir's failure to list the deck directory.
type DirError struct{ Err error }

func (e *DirError) Error() string { return e.Err.Error() }
func (e *DirError) Unwrap() error { return e.Err }
