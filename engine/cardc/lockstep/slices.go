package lockstep

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/rules"
)

const fdnDir = "dirac/oracle/testdata/slices/fdn"

// FDN seats pairs [from, from+n) of dirac/oracle/testdata/slices/fdn/pairs.tsv,
// read by path from root. Out-of-range bounds are clamped.
func FDN(root string, forge *cards.Registry, from, n int) ([]Game, error) {
	pairsPath := filepath.Join(root, fdnDir, "pairs.tsv")
	pf, err := os.Open(pairsPath)
	if err != nil {
		return nil, err
	}
	defer pf.Close()
	type pair struct {
		a, b string
		seed uint64
	}
	var pairs []pair
	sc := bufio.NewScanner(pf)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 3 {
			return nil, fmt.Errorf("%s: line %d: want 3 fields", pairsPath, len(pairs)+1)
		}
		seed, err := strconv.ParseUint(f[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s: line %d: %w", pairsPath, len(pairs)+1, err)
		}
		pairs = append(pairs, pair{f[0], f[1], seed})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if from < 0 {
		from = 0
	}
	if from > len(pairs) {
		from = len(pairs)
	}
	if n < 0 {
		n = 0
	}
	if from+n > len(pairs) {
		n = len(pairs) - from
	}
	pairs = pairs[:from+n]
	need := map[string]bool{}
	for _, p := range pairs[from:] {
		need[p.a], need[p.b] = true, true
	}
	raw, err := readDecks(filepath.Join(root, fdnDir, "decks.tar.gz"), need)
	if err != nil {
		return nil, err
	}
	resolved := map[string][]*cards.Card{}
	load := func(name string) ([]*cards.Card, error) {
		if d, ok := resolved[name]; ok {
			return d, nil
		}
		f, err := deck.Parse(raw[name])
		if err != nil {
			return nil, fmt.Errorf("deck %s: %w", name, err)
		}
		d, err := f.Resolve(forge)
		if err != nil {
			return nil, fmt.Errorf("deck %s: %w", name, err)
		}
		resolved[name] = d
		return d, nil
	}
	games := make([]Game, 0, n)
	for i := from; i < from+n; i++ {
		p := pairs[i]
		a, err := load(p.a)
		if err != nil {
			return nil, err
		}
		b, err := load(p.b)
		if err != nil {
			return nil, err
		}
		games = append(games, Game{ID: fmt.Sprintf("fdn/%d", i), Cfg: rules.Config{
			Seed: p.seed, Names: []string{p.a, p.b}, Decks: [][]*cards.Card{a, b}, Tokens: forge.Tokens,
			NameUniverse: forge.Universe(), Mulligans: 1, LoopGuard: rules.WithDefaultObjectCap(nil)}})
	}
	return games, nil
}

// readDecks returns decks/<name>.json for each wanted name.
func readDecks(path string, want map[string]bool) (map[string][]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	tr := tar.NewReader(zr)
	out := map[string][]byte{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		name, ok := strings.CutPrefix(h.Name, "decks/")
		if !ok {
			continue
		}
		name, ok = strings.CutSuffix(name, ".json")
		if !ok || !want[name] {
			continue
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		out[name] = b
	}
	for n := range want {
		if _, ok := out[n]; !ok {
			return nil, fmt.Errorf("%s: no deck %s", path, n)
		}
	}
	return out, nil
}

// Legacy seats the acceptance game at len(decks) seats from already-resolved decks.
func Legacy(forge *cards.Registry, names []string, decks [][]*cards.Card) Game {
	return Game{ID: fmt.Sprintf("legacy/%d", len(decks)), Cfg: rules.AcceptanceConfig(forge, names, decks)}
}
