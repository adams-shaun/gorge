package cardset_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// hostedDecksDir is where gorged's hosted decks live, from this package.
var hostedDecksDir = filepath.Join("..", "..", "testkit", "testutil", "decks")

// TestHostedCommanderPoolCoversRepoDecks: pools/hosted-cmdr/cards.txt is the
// union of every card (commander included) of the testkit/testutil/decks files
// whose format is commander. adapter.table plays a pod from one set, so a deck
// whose card sits in no pool (or in a different pool from the rest) never plays.
// A deck that gains a card fails here until the list is regenerated and
// `make dirac-tables` writes ids.tsv.
func TestHostedCommanderPoolCoversRepoDecks(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(hostedDecksDir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no repo decks under %s (err %v)", hostedDecksDir, err)
	}
	want := map[string]string{} // card -> the first deck holding it
	decks := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var d struct {
			Format     string   `json:"format"`
			Commander  string   `json:"commander"`
			Commanders []string `json:"commanders"`
			Cards      []struct {
				Name string `json:"name"`
			} `json:"cards"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if d.Format != "commander" {
			continue
		}
		decks++
		deck := filepath.Base(f)
		names := append([]string{d.Commander}, d.Commanders...)
		for _, c := range d.Cards {
			names = append(names, c.Name)
		}
		for _, n := range names {
			if n == "" {
				continue
			}
			if _, ok := want[n]; !ok {
				want[n] = deck
			}
		}
	}
	// The pool exists to hold whole decks; a glob that found none would pass vacuously.
	if decks < 19 {
		t.Fatalf("%d commander decks under %s, want at least the 19 the pool was built from", decks, hostedDecksDir)
	}

	raw, err := os.ReadFile(filepath.Join("pools", "hosted-cmdr", "cards.txt"))
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, l := range strings.Split(string(raw), "\n") {
		if n := strings.TrimSpace(l); n != "" {
			have[n] = true
		}
	}
	var missing, extra []string
	for n, deck := range want {
		if !have[n] {
			missing = append(missing, n+" ("+deck+")")
		}
	}
	for n := range have {
		if _, ok := want[n]; !ok {
			extra = append(extra, n)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		t.Errorf("%d card(s) of a repo commander deck are missing from pools/hosted-cmdr/cards.txt: %s",
			len(missing), strings.Join(missing, "; "))
	}
	if len(extra) > 0 {
		t.Errorf("%d card(s) of pools/hosted-cmdr/cards.txt are in no repo commander deck: %s",
			len(extra), strings.Join(extra, "; "))
	}
	if !sort.StringsAreSorted(strings.Split(strings.TrimRight(string(raw), "\n"), "\n")) {
		t.Error("pools/hosted-cmdr/cards.txt is not sorted")
	}

	// The registered pool is served and holds every card. Without a corpus
	// there is no table, and the list check above stands alone.
	set, err := cardset.ByName("hosted-cmdr")
	if errors.Is(err, cardset.ErrNoCorpus) {
		return
	}
	if err != nil {
		t.Fatalf("ByName(hosted-cmdr): %v", err)
	}
	var unserved []string
	for n := range want {
		if _, ok := set.Lookup(n); !ok {
			unserved = append(unserved, n)
		}
	}
	sort.Strings(unserved)
	if len(unserved) > 0 {
		t.Errorf("hosted-cmdr serves no entry for %d card(s): %s", len(unserved), strings.Join(unserved, "; "))
	}
}
