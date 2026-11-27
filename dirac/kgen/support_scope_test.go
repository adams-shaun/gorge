package kgen

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// scopeListPath is the committed out-of-scope list (scripts/dirac-scope-list.py writes it):
// the corpus cards the support page's in-scope columns leave out of the denominator.
const scopeListPath = "dirac/support/scope/out-of-scope.txt"

// scopeReasons are the reasons a scope-list row may carry.
var scopeReasons = []string{
	"digital-only", "alchemy-rebalanced", "not-commander-legal", "non-tournament-type",
	"non-tournament-mechanic", "deferred-mutate", "deferred-splice",
}

// scopeRow is one line of the scope list.
type scopeRow struct{ name, reason string }

// readScopeList reads the scope list from the module root, in file order.
func readScopeList(t *testing.T) []scopeRow {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(testRoot, scopeListPath))
	if err != nil {
		t.Fatal(err)
	}
	var rows []scopeRow
	for i, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		name, reason, ok := strings.Cut(line, "\t")
		if !ok || name == "" || strings.Contains(reason, "\t") {
			t.Fatalf("%s:%d: %q is not name<TAB>reason", scopeListPath, i+1, line)
		}
		rows = append(rows, scopeRow{name, reason})
	}
	return rows
}

// outOfScope is the scope list as a set of names.
func outOfScope(rows []scopeRow) map[string]bool {
	m := make(map[string]bool, len(rows))
	for _, r := range rows {
		m[r.name] = true
	}
	return m
}

// inScope reports whether a lowered card is in scope: the list names a card as the corpus
// does, by its first face, so a split card is looked up by that face too.
func inScope(out map[string]bool, name string) bool {
	return !out[name] && !out[firstFace(name)]
}

// TestScopeListIsWellFormed: the scope list is sorted bytewise with no name twice, every reason
// is one of scopeReasons, and (with a corpus) every name is a corpus card.
func TestScopeListIsWellFormed(t *testing.T) {
	rows := readScopeList(t)
	if len(rows) == 0 {
		t.Fatalf("%s is empty", scopeListPath)
	}
	for i, r := range rows {
		if !slices.Contains(scopeReasons, r.reason) {
			t.Errorf("%s: %q has reason %q, not one of %v", scopeListPath, r.name, r.reason, scopeReasons)
		}
		if i > 0 && rows[i-1].name >= r.name {
			t.Errorf("%s: %q after %q: not sorted, or a name twice", scopeListPath, r.name, rows[i-1].name)
		}
	}
	all, err := corpusAllNames(filepath.Join(ratchetCards(t), "cardsfolder"))
	if err != nil {
		t.Fatal(err)
	}
	corpus := map[string]bool{}
	for _, n := range bytes.Split(bytes.TrimSuffix(all, []byte("\n")), []byte("\n")) {
		corpus[string(n)] = true
	}
	if !corpus["Lightning Bolt"] {
		t.Fatal("the corpus names hold no Lightning Bolt: not a corpus")
	}
	for _, r := range rows {
		if !corpus[r.name] {
			t.Errorf("%s: %q is not a corpus card", scopeListPath, r.name)
		}
	}
}

// TestSupportPageInScopeColumns: a rendered row carries the three in-scope columns after
// pod-evidenced and parses back; a page from before them still parses, its in-scope zero.
func TestSupportPageInScopeColumns(t *testing.T) {
	out := outOfScope([]scopeRow{{"Cease", "digital-only"}, {"Dairy Cow", "not-commander-legal"}})
	if inScope(out, "Cease // Desist") || inScope(out, "Dairy Cow") || !inScope(out, "Lightning Bolt") {
		t.Error("inScope misreads the list")
	}
	r := supportCounts{list: "BLB", cards: 278, lowered: 261, refused: 2, supported: 259, fourSeat: 258, podEvidenced: 1, inScope: 270, inScopeLowered: 255, inScopeSupported: 250}
	row := supportRow(r)
	if want := "| BLB | 278 | 261 | 2 | 0 | 259 | 258 | 1 | 270 | 255 | 250 |\n"; row != want {
		t.Fatalf("row %q, want %q", row, want)
	}
	page := func(rows string) []byte { return []byte(rows + "\n## Refused by name\n\nNone.\n") }
	p, err := parseSupportDoc(page(row))
	if err != nil || len(p.rows) != 1 || p.rows[0] != r {
		t.Fatalf("parsed %+v, %v; want %+v", p.rows, err, r)
	}
	p, err = parseSupportDoc(page("| BLB | 278 | 261 | 2 | 0 | 259 | 258 | 1 |\n"))
	if err != nil || len(p.rows) != 1 || p.rows[0].inScope != 0 || p.rows[0].podEvidenced != 1 {
		t.Fatalf("an old-layout page: %+v, %v", p.rows, err)
	}
}
