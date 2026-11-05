package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cli"
	"github.com/adams-shaun/gorge/draft/draftlab"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func deckSize(f deck.File) int {
	n := 0
	for _, e := range f.Cards {
		n += e.Count
	}
	return n
}

func readManifest(t *testing.T, dir string) draftlab.Manifest {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m draftlab.Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	return m
}

func readIndex(t *testing.T, dir string) []indexEntry {
	t.Helper()
	f, err := os.Open(filepath.Join(dir, "decks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []indexEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e indexEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("index line %q: %v", sc.Text(), err)
		}
		out = append(out, e)
	}
	return out
}

func parseDeckFiles(t *testing.T, dir string, want int) []deck.File {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "decks", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != want {
		t.Fatalf("decks/ holds %d files, want %d", len(paths), want)
	}
	var out []deck.File
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f, err := deck.Parse(raw)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if f.Format != "limited" {
			t.Errorf("%s: format %q, want limited", p, f.Format)
		}
		if n := deckSize(f); n != 40 {
			t.Errorf("%s: %d cards, want 40", p, n)
		}
		out = append(out, f)
	}
	return out
}

func TestDeckpoolWritesLoadableDecks(t *testing.T) {
	corpus := testutil.CorpusDirOrSkip(t)
	out := filepath.Join(t.TempDir(), "pool")
	var stdout bytes.Buffer
	o := options{set: "FDN", pods: 1, seed: 7, drafter: "rating-colour", builder: "best23", corpus: corpus, out: out, quiet: true}
	if err := run(o, &stdout); err != nil {
		t.Fatal(err)
	}
	if want := "deckpool: wrote 8 decks to " + out + "\n"; stdout.String() != want {
		t.Errorf("stdout %q, want %q", stdout.String(), want)
	}
	lab, err := draftlab.Open(corpus, draftlab.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range parseDeckFiles(t, out, 8) {
		if _, err := lab.ListDeck(f); err != nil {
			t.Errorf("ListDeck %s: %v", f.Name, err)
		}
	}
	idx := readIndex(t, out)
	if len(idx) != 8 {
		t.Fatalf("index has %d lines, want 8", len(idx))
	}
	for seat, e := range idx {
		if e.Seat != seat || e.Seed != 7 || e.DraftID != "7-s"+string(rune('0'+seat)) {
			t.Errorf("line %d = %+v", seat, e)
		}
		if _, err := os.Stat(filepath.Join(out, e.File)); err != nil {
			t.Errorf("line %d file: %v", seat, err)
		}
	}
	m := readManifest(t, out)
	if m.Kind != "deckpool" || m.Set != "FDN" || m.Seats != 8 || m.Mode != "draft" || m.Collation == "" {
		t.Errorf("manifest = %+v", m)
	}
	if want := []string{"builder:best23@1", "drafter:rating-colour@1"}; !reflect.DeepEqual(m.Agents, want) {
		t.Errorf("agents = %v, want %v", m.Agents, want)
	}
}

func trkOptions(corpus, out string, seed uint64) options {
	return options{set: "TRK", pods: 2, seed: seed, drafter: "balanced", builder: "balanced", corpus: corpus, out: out}
}

func TestDeckpoolDraftsTRK(t *testing.T) {
	corpus := testutil.CorpusDirOrSkip(t)
	out := filepath.Join(t.TempDir(), "pool")
	if err := run(trkOptions(corpus, out, 1000), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	parseDeckFiles(t, out, 16)
	idx := readIndex(t, out)
	if len(idx) != 16 || idx[0].Seed != 1000 || idx[8].Seed != 1001 {
		t.Errorf("index seeds wrong: %d lines", len(idx))
	}
	m := readManifest(t, out)
	if want := []string{"builder:balanced@1", "drafter:balanced@1"}; !reflect.DeepEqual(m.Agents, want) {
		t.Errorf("agents = %v, want %v", m.Agents, want)
	}
}

// readTree maps every file's relative path under dir to its bytes.
func readTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		tree[rel] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestDeckpoolIsDeterministic(t *testing.T) {
	corpus := testutil.CorpusDirOrSkip(t)
	base := t.TempDir()
	for _, c := range []struct {
		dir  string
		seed uint64
	}{{"a", 1000}, {"b", 1000}, {"c", 1001}} {
		if err := run(trkOptions(corpus, filepath.Join(base, c.dir), c.seed), &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
	}
	a, b, c := readTree(t, filepath.Join(base, "a")), readTree(t, filepath.Join(base, "b")), readTree(t, filepath.Join(base, "c"))
	if len(a) != 16+2 {
		t.Fatalf("tree has %d files, want 18", len(a))
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("same flags wrote different trees")
	}
	if a["decks.jsonl"] == c["decks.jsonl"] {
		t.Errorf("seed 1001 wrote the same decks.jsonl as seed 1000")
	}
}

func TestDeckpoolRefusesNonEmptyOut(t *testing.T) {
	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(out, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Refused before the corpus opens, so a bogus corpus proves "before any work".
	err := run(options{set: "TRK", pods: 1, seed: 1, drafter: "balanced", builder: "balanced", corpus: "/nonexistent", out: out}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--out") {
		t.Fatalf("err = %v, want one naming --out", err)
	}
	entries, _ := os.ReadDir(out)
	if len(entries) != 1 || entries[0].Name() != "x" {
		t.Errorf("out holds %v, want only x", entries)
	}
}

func TestDeckpoolFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Execute(newRoot(), []string{"--pods", "0", "--out", t.TempDir()}, nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("--pods 0: exit %d, want 2 (stderr %q)", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := cli.Execute(newRoot(), []string{"--print-config"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("--print-config: exit %d (stderr %q)", code, stderr.String())
	}
	for _, want := range []string{`"set": "TRK"`, `"pods": 125`, `"drafter": "balanced"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("--print-config lacks %s:\n%s", want, stdout.String())
		}
	}
}
