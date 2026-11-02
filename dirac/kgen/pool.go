package kgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// deckFile is one deck of the pool directory: {name, cards:[{name,count}]}.
type deckFile struct {
	Name  string `json:"name"`
	Cards []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"cards"`
}

// readCardList returns a card list's names (one card name per line, blank
// lines and # comments ignored), deduplicated in file order.
func readCardList(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseCardList(b), nil
}

// parseCardList is readCardList over a list's bytes.
func parseCardList(b []byte) []string {
	seen := map[string]bool{}
	var names []string
	for _, line := range strings.Split(string(b), "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		name := strings.TrimSpace(line)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

// loadCardList builds a pool from a card list instead of deck files. There
// are no decks, so nDecks is 1 for every card and coverage reports cards only.
func loadCardList(path, cardsDir string) (*pool, error) {
	names, err := readCardList(path)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no cards in %s", path)
	}
	return loadCardNames(names, cardsDir)
}

// loadCardNames builds a pool from already-read card names.
func loadCardNames(names []string, cardsDir string) (*pool, error) {
	sort.Strings(names)
	p := &pool{nDecks: map[string]int{}, byName: map[string]*cards.Card{}}
	reg, err := cards.OpenCorpusSubset(cardsDir, names)
	if err != nil {
		return nil, err
	}
	if p.typeWords, err = corpusTypeWords(cardsDir); err != nil {
		return nil, err
	}
	p.reg = reg
	for _, n := range names {
		c, ok := reg.Lookup(n)
		if !ok {
			return nil, fmt.Errorf("card %q not in the corpus", n)
		}
		p.names = append(p.names, n)
		p.nDecks[n] = 1
		p.byName[n] = c
	}
	return p, nil
}

// pool is the deck pool and the compiled cards it names.
type pool struct {
	decks     [][]string // per deck, its distinct card names, sorted
	deckNames []string
	names     []string // every distinct card name, sorted
	nDecks    map[string]int
	reg       *cards.Registry
	byName    map[string]*cards.Card
	subs      map[string]bool // knownSubtype's lazily built set
	typeWords map[string]bool // the corpus's type vocabulary (corpusTypeWords); nil on a synthetic pool
}

// loadPool reads every *.json deck in decksDir and resolves its cards through
// the compiled corpus at cardsDir. An unresolved name is an error: the
// generated table would silently lack it.
func loadPool(decksDir, cardsDir string) (*pool, error) {
	paths, err := filepath.Glob(filepath.Join(decksDir, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no decks in %s", decksDir)
	}
	sort.Strings(paths)
	p := &pool{nDecks: map[string]int{}, byName: map[string]*cards.Card{}}
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var d deckFile
		if err := json.Unmarshal(b, &d); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		seen := map[string]bool{}
		var names []string
		for _, c := range d.Cards {
			if !seen[c.Name] {
				seen[c.Name] = true
				names = append(names, c.Name)
				p.nDecks[c.Name]++
			}
		}
		sort.Strings(names)
		p.decks = append(p.decks, names)
		p.deckNames = append(p.deckNames, d.Name)
	}
	for n := range p.nDecks {
		p.names = append(p.names, n)
	}
	sort.Strings(p.names)
	reg, err := cards.OpenCorpusSubset(cardsDir, p.names)
	if err != nil {
		return nil, err
	}
	if p.typeWords, err = corpusTypeWords(cardsDir); err != nil {
		return nil, err
	}
	p.reg = reg
	for _, n := range p.names {
		c, ok := reg.Lookup(n)
		if !ok {
			return nil, fmt.Errorf("card %q not in the corpus", n)
		}
		p.byName[n] = c
	}
	return p, nil
}
