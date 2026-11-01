package oracle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
)

// The deck-pair selection gorge self-play and self-play share: the pool pairings
// (fuzz.go PairingsFrom), an explicit -pairs-file, the -only allowed-deck
// list and the -deckfile inline decks. Both runners call these, so their
// pairing and selection cannot drift.

// PairSpec is one -pairs-file line: deckA, deckB and the line's optional
// seed (SeedText "" derives the seed from the run's -seed).
type PairSpec struct {
	A, B     string
	SeedText string
}

// ExplicitPairs reads a -pairs-file: deckA, deckB
// and an optional seed per tab-separated line; blank lines and lines starting
// with # are skipped.
func ExplicitPairs(path string) ([]PairSpec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// one PairSpec a line, allocated once (a 200k-line file's list
	// would otherwise be grown by doubling), and the lines walked in place
	text := string(raw)
	out := make([]PairSpec, 0, strings.Count(text, "\n")+1)
	i := -1
	for rest := text; rest != ""; {
		var l string
		l, rest, _ = strings.Cut(rest, "\n")
		i++
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		// two or three tab-separated fields, cut in place
		a, rest, two := strings.Cut(l, "\t")
		b, seed, three := strings.Cut(rest, "\t")
		if !two || three && strings.Contains(seed, "\t") || a == "" || b == "" {
			return nil, fmt.Errorf("%s line %d: want deckA<TAB>deckB[<TAB>seed]", path, i+1)
		}
		out = append(out, PairSpec{A: a, B: b, SeedText: seed})
	}
	return out, nil
}

// PairSeed is pair p's game seed: the -pairs-file line's own, else
// PCG(seed, p)'s next uint64 (the explicit-pair rule; the pool
// rule, PairingsFrom, draws the deck indexes from the same stream first).
func PairSeed(line PairSpec, p int, seed uint64) (uint64, error) {
	r := rand.New(rand.NewPCG(seed, uint64(p)))
	s := r.Uint64()
	if line.SeedText != "" {
		v, err := strconv.ParseUint(line.SeedText, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("seed %q: %w", line.SeedText, err)
		}
		s = v
	}
	return s, nil
}

// OnlyPool filters pool to the decks an -only list allows:
// the list's trimmed lines are the allowed names.
func OnlyPool(pool []PoolDeck, path string) ([]PoolDeck, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ok := map[string]bool{}
	for _, l := range bytes.Split(raw, []byte("\n")) {
		ok[string(bytes.TrimSpace(l))] = true
	}
	kept := pool[:0]
	for _, p := range pool {
		if ok[p.Name] {
			kept = append(kept, p)
		}
	}
	return kept, nil
}

// ReadDeckFile reads a -deckfile: one gorge
// deck object a line, keyed by its "name".
func ReadDeckFile(path string) (map[string][]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	for i, l := range bytes.Split(raw, []byte("\n")) {
		l = bytes.TrimSpace(l)
		if len(l) == 0 {
			continue
		}
		var h struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(l, &h); err != nil || h.Name == "" {
			return nil, fmt.Errorf("%s line %d: a deck needs a name (%v)", path, i+1, err)
		}
		if _, dup := out[h.Name]; dup {
			return nil, fmt.Errorf("%s line %d: deck %q listed twice", path, i+1, h.Name)
		}
		out[h.Name] = l
	}
	return out, nil
}
