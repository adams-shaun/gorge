package oracle

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/deck"
)

// stdMetaCards is the committed card list of the standard-meta card set.
func stdMetaCards(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "cardset", "pools", "standard-meta", "cards.txt"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, l := range strings.Split(string(raw), "\n") {
		if l != "" {
			out[l] = true
		}
	}
	return out
}

// TestStdMetaPoolTree: where the pool tree is on disk (GORGE_POOLS/standard-meta),
// it is the one the registry pins: its digest, 60 decks of 60 or 70 cards, every card
// in the card set's list.
func TestStdMetaPoolTree(t *testing.T) {
	p, err := PoolByName("standard-meta")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.Inputs, "pool.tsv")); err != nil {
		t.Skipf("no standard-meta pool tree at %s (set GORGE_POOLS): %v", p.Inputs, err)
	}
	got, err := TreeDigest(p.Inputs)
	if err != nil {
		t.Fatal(err)
	}
	if got != p.Tree {
		t.Errorf("tree digest %s, registered %s", got, p.Tree)
	}
	rows, err := ReadPool(filepath.Join(p.Inputs, "pool.tsv"), "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 60 {
		t.Fatalf("%d decks in pool.tsv, want 60", len(rows))
	}
	cards := stdMetaCards(t)
	sizes := map[int]int{}
	for _, r := range rows {
		raw, err := os.ReadFile(filepath.Join(p.Inputs, "decks", r.Name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		f, err := deck.Parse(raw)
		if err != nil {
			t.Fatalf("%s: %v", r.Name, err)
		}
		n := 0
		for _, e := range f.Cards {
			n += e.Count
			if !cards[e.Name] {
				t.Errorf("%s: %q is not in standard-meta's cards.txt", r.Name, e.Name)
			}
		}
		if n != 60 && n != 70 {
			t.Errorf("%s has %d cards, want 60 or 70", r.Name, n)
		}
		sizes[n]++
	}
	if sizes[60] != 59 || sizes[70] != 1 {
		t.Errorf("deck sizes %v, want 59 of 60 and 1 of 70", sizes)
	}
}

// stdMetaPairRefusal is why the kernel refuses a pair of decks that are fine alone, or
// "" for no refusal: a cardset.InteractionRefusal, or the untyped layer, Room-name and
// cost-reduction refusals kfuzz counts the same way (kfuzz skipClass).
func stdMetaPairRefusal(err error) string {
	if err == nil {
		return ""
	}
	var ir *cardset.InteractionRefusal
	if errors.As(err, &ir) {
		return "interaction " + ir.Reason
	}
	for _, p := range []string{"layer guard: ", "Room name read: ", "ReduceCost Type Ability beside "} {
		if strings.HasPrefix(err.Error(), p) {
			return strings.TrimRight(p, ": ")
		}
	}
	return ""
}

// TestStdMetaPairMatchup builds the kernel table of every pair of the committed
// slice: no real pair's subtypes may pass kop.MaxSubtypes (the whole pool's 146
// are past the old 128). A pair the kernel refuses for a card interaction is counted, and one the kernel
// refuses for any other reason fails.
func TestStdMetaPairMatchup(t *testing.T) {
	set, err := cardset.ByName("standard-meta")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no standard-meta table (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.AllowUnverified(set) // the new-pool mode: kfuzz -allow-unverified
	dir, remove, err := ExtractSlice("standard-meta")
	if err != nil {
		t.Fatal(err)
	}
	defer remove()
	lines, err := ExplicitPairs(filepath.Join(dir, "pairs.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != SlicePairs {
		t.Fatalf("%d pairs, want %d", len(lines), SlicePairs)
	}
	entries := map[string][]cardset.Entry{}
	of := func(name string) []cardset.Entry {
		if es, ok := entries[name]; ok {
			return es
		}
		raw, err := os.ReadFile(filepath.Join(dir, "decks", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		f, err := deck.Parse(raw)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var es []cardset.Entry
		for _, e := range f.Cards {
			es = append(es, cardset.Entry{Name: e.Name, Count: e.Count})
		}
		entries[name] = es
		return es
	}
	type pair struct{ a, b string }
	done := map[pair]bool{}
	built, refused, most := 0, 0, 0
	reasons := map[string]int{}
	for _, l := range lines {
		k := pair{l.A, l.B}
		if done[k] {
			continue
		}
		done[k] = true
		tbl, _, _, err := set.Matchup(of(l.A), of(l.B))
		if why := stdMetaPairRefusal(err); why != "" {
			refused++
			reasons[why]++
			continue
		}
		if err != nil {
			t.Fatalf("%s vs %s: %v", l.A, l.B, err)
		}
		built++
		if n := len(tbl.Subtypes); n > kop.MaxSubtypes {
			t.Errorf("%s vs %s: %d subtypes, past %d", l.A, l.B, n, kop.MaxSubtypes)
		} else if n > most {
			most = n
		}
	}
	t.Logf("%d distinct pairs: %d built (most subtypes %d), %d refused as interactions %v", len(done), built, most, refused, reasons)
	if built == 0 {
		t.Fatal("no pair built a table")
	}
}
