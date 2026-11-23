package cardset

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// refusedDeckTolerated lists (pool, card) pairs where a committed oracle deck
// holds a card dirac/support/refused names, and the oracle's tests are known to
// skip such decks instead of failing: the mbc slice feeds mbcpods_test.go,
// which counts a pod holding a refused card as "refused" and moves on
// (errors.As RefusedError, StatusRefused). Nothing else may be listed: the
// four-seat Commander census (podtwoseat_census_mp_test.go, pools cmd5 and
// edh) fatals on a deck it cannot build.
var refusedDeckTolerated = map[string]map[string]bool{
	"mbc": {"Nylea's Presence": true, "Smiting Helix": true},
}

type deckJSON struct {
	Cards []struct {
		Name string `json:"name"`
	} `json:"cards"`
}

// TestNoRefusedCardInAFixedOracleDeck stops the 11-23 oracle reds: five PRs
// (#3779 #3797 #3805 #3807 #3819) each added a dirac/support/refused entry whose
// own checks passed, while a card of it sat in a committed deck a dirac/oracle
// lockstep test plays (the four-seat Commander census fatals on it), so main's
// whole oracle went red (revert #3900). The oracle only runs on main; this reads
// the refusal files and the committed decks (slice tarballs and the giada decks)
// and plays no game.
func TestNoRefusedCardInAFixedOracleDeck(t *testing.T) {
	refused := map[string]string{} // card -> refusal file
	files, err := filepath.Glob(filepath.Join("..", "support", "refused", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no refusal files found (err %v)", err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var e RefusedEntry
		if err := json.Unmarshal(b, &e); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		refused[e.Card] = "dirac/support/refused/" + filepath.Base(f)
	}
	var bad []string
	check := func(pool, deck string, b []byte) {
		var d deckJSON
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatalf("%s %s: %v", pool, deck, err)
		}
		for _, c := range d.Cards {
			src, ok := refused[c.Name]
			if !ok || refusedDeckTolerated[pool][c.Name] {
				continue
			}
			bad = append(bad, "card "+c.Name+" is in deck "+deck+" ("+pool+") but refused by "+src+
				": swap it for basics in the deck file (as the precon notes do), or refuse it in the same change that fixes the decks")
		}
	}
	slices, err := filepath.Glob(filepath.Join("..", "oracle", "testdata", "slices", "*", "decks.tar.gz"))
	if err != nil || len(slices) == 0 {
		t.Fatalf("no committed slices found (err %v)", err)
	}
	for _, s := range slices {
		pool := filepath.Base(filepath.Dir(s))
		f, err := os.Open(s)
		if err != nil {
			t.Fatal(err)
		}
		gz, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		tr := tar.NewReader(gz)
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("%s: %v", s, err)
			}
			if !strings.HasSuffix(h.Name, ".json") {
				continue
			}
			b, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			check(pool, "oracle/testdata/slices/"+pool+"/"+h.Name, b)
		}
		f.Close()
	}
	giada, _ := filepath.Glob(filepath.Join("..", "oracle", "testdata", "giada", "decks", "*.json"))
	for _, g := range giada {
		b, err := os.ReadFile(g)
		if err != nil {
			t.Fatal(err)
		}
		check("giada", "oracle/testdata/giada/decks/"+filepath.Base(g), b)
	}
	sort.Strings(bad)
	for _, m := range bad {
		t.Error(m)
	}
	// A stale allowlist entry hides nothing but rots: it must still be refused.
	for pool, m := range refusedDeckTolerated {
		for c := range m {
			if _, ok := refused[c]; !ok {
				t.Errorf("refusedDeckTolerated[%s] lists %s, which is no longer refused: delete it", pool, c)
			}
		}
	}
}
