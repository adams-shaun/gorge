package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testSetlistPath is a registered list's path for a test run from dirac/kgen. ALL
// is generated from the corpus; every other list is a tracked file.
func testSetlistPath(t *testing.T, list string) string {
	t.Helper()
	if list != "ALL" {
		return filepath.Join(testSetlistDir, list+".txt")
	}
	path, err := corpusAllList(ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// TestSetlistAllMatchesCorpus: at this FORGE_REF the generated ALL list holds as
// many names as the ALL pin lists, in strict byte order.
func TestSetlistAllMatchesCorpus(t *testing.T) {
	b, err := os.ReadFile(testSetlistPath(t, "ALL"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.HasSuffix(s, "\n") {
		t.Fatal("ALL does not end in a newline")
	}
	names := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if want := acceptancePins["ALL"][1]; len(names) != want {
		t.Errorf("ALL lists %d names, the pin %d", len(names), want)
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Fatalf("ALL line %d: %q is not byte-after %q", i+1, names[i], names[i-1])
		}
	}
}

// TestCorpusAllList pins the list's semantics on a small corpus: a script's first
// Name: only, CR stripped, duplicates folded, byte order (an uppercase name sorts
// before a lowercase one), a non-script or nameless file ignored, and an unchanged
// list left unwritten.
func TestCorpusAllList(t *testing.T) {
	cards := t.TempDir()
	for name, body := range map[string]string{
		"a/front.txt":  "Name:Bravo Front\r\nTypes:Creature\r\n\r\nALTERNATE\r\n\r\nName:Bravo Back\r\n",
		"a/dup.txt":    "Name:Alpha\nTypes:Sorcery\n",
		"b/dup2.txt":   "Name:Alpha\nTypes:Instant\n",
		"b/lower.txt":  "Types:Land\nName:alpha lower\n",
		"b/noname.txt": "Types:Land\n",
		"b/notes.md":   "Name:Not A Script\n",
		"c/d/deep.txt": "Name:Zulu\n",
		"c/indent.txt": " Name:Indented\nName:Yankee\n",
	} {
		p := filepath.Join(cards, "cardsfolder", name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	path, err := corpusAllList(cards)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(cards, "kgen", "ALL.txt"); path != want {
		t.Errorf("path %s, want %s", path, want)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Alpha\nBravo Front\nYankee\nZulu\nalpha lower\n"; string(got) != want {
		t.Errorf("list %q, want %q", got, want)
	}

	old := time.Unix(1000, 0)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := corpusAllList(cards); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !st.ModTime().Equal(old) {
		t.Error("an unchanged list was rewritten")
	}

	// A stale list is replaced, and the temp file does not stay behind.
	if err := os.WriteFile(path, []byte("Stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := corpusAllList(cards); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); strings.Contains(string(got), "Stale") {
		t.Errorf("a stale list survived: %q", got)
	}
	ents, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 {
		t.Errorf("kgen dir holds %d entries, want the list alone", len(ents))
	}

	if _, err := corpusAllList(t.TempDir()); err == nil {
		t.Error("a corpus with no cardsfolder listed cards")
	}
}
