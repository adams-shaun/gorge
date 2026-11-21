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

// cardRow is one row of a card list: the card's name and, when the line carries
// one, the text after the first tab — an evidence row's seats column (Name\t<seats>,
// what scripts/dirac-first-refusals.py evidence --pools pod-* lands the card's clean
// pod games in).
type cardRow struct {
	name string
	col  string
}

// parseCardRows reads a list's rows as cardRows.
func parseCardRows(b []byte) []cardRow {
	seen := map[string]bool{}
	var rows []cardRow
	for _, line := range strings.Split(string(b), "\n") {
		name, col, _ := strings.Cut(line, "\t")
		if i := strings.IndexByte(name, '#'); i >= 0 {
			name = name[:i]
		}
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		rows = append(rows, cardRow{name: name, col: col})
	}
	return rows
}

// parseCardList is readCardList over a list's bytes. A row's trailing
// tab-separated columns are not part of the name; readers that split by tab
// ignore them.
func parseCardList(b []byte) []string {
	var names []string
	for _, r := range parseCardRows(b) {
		names = append(names, r.name)
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

// poolCorpus is the corpus a pool loads against: the process's shared corpus
// registry (cards.SharedCorpus) and the corpus's type vocabulary. A pool
// reads only Lookup, Token and Tokens of its registry, and a subset registry
// answers those exactly as the shared full one (subset.go: a subset card is
// the value the full registry holds; the tokens are all of them either way),
// so every pool of a build loads against the one shared open instead of one
// OpenCorpusSubset per pool. That hoists the staleness rule -- the corpus
// walk, the token digest and the card census sidecars (cacheFresh, the one
// home of the rule; OpenCorpus and the segment path both consult it) -- out
// of the per-pool loop: 899 pools re-ran it in `kgen -tables`, 93% of the
// build, and the synthetic pools' build re-ran it per list and per split.
type poolCorpus struct {
	reg       *cards.Registry
	typeWords map[string]bool
}

// openPoolCorpus opens the process's shared corpus for cardsDir. Both parts
// are memoised per directory (cards.SharedCorpus, corpusWords), so a build
// calls this once and later calls, including the next build in the same
// process, are free.
func openPoolCorpus(cardsDir string) (*poolCorpus, error) {
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		return nil, err
	}
	tw, err := corpusTypeWords(cardsDir)
	if err != nil {
		return nil, err
	}
	return &poolCorpus{reg: reg, typeWords: tw}, nil
}

// cardPool is the pool side of loadCardNames: names, already sorted, against
// an open corpus. A name the corpus does not resolve is an error: the
// generated table would silently lack it.
func cardPool(names []string, pc *poolCorpus) (*pool, error) {
	p := &pool{nDecks: map[string]int{}, byName: map[string]*cards.Card{}, reg: pc.reg, typeWords: pc.typeWords}
	for _, n := range names {
		c, ok := pc.reg.Lookup(n)
		if !ok {
			return nil, fmt.Errorf("card %q not in the corpus", n)
		}
		p.names = append(p.names, n)
		p.nDecks[n] = 1
		p.byName[n] = c
	}
	return p, nil
}

// loadCardNames builds a pool from already-read card names.
func loadCardNames(names []string, cardsDir string) (*pool, error) {
	sort.Strings(names)
	pc, err := openPoolCorpus(cardsDir)
	if err != nil {
		return nil, err
	}
	return cardPool(names, pc)
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

	// allBlockers makes lowering keep reading past a refusal and record
	// every one it can reach (-census-std -all-blockers). The lowered
	// output of a card that lowers cleanly is the same either way.
	allBlockers bool

	// commander: the pool's games are Commander pods (tables.go readPool
	// sets it by pool name); a Meld result card refuses in one (lower_meld.go).
	commander bool
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
	pc, err := openPoolCorpus(cardsDir)
	if err != nil {
		return nil, err
	}
	p.reg, p.typeWords = pc.reg, pc.typeWords
	for _, n := range p.names {
		c, ok := pc.reg.Lookup(n)
		if !ok {
			return nil, fmt.Errorf("card %q not in the corpus", n)
		}
		p.byName[n] = c
	}
	return p, nil
}
