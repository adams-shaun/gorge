package oracle

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestReproCompactRoundTrip guards the corpus compaction (cleanup B1): every
// recorded repro, re-encoded as the compact JSON the recordings now use — the
// derived `readable` field dropped, no indentation — and decoded again, keeps
// the Actions, Cards and Seed the file on disk decodes to.
func TestReproCompactRoundTrip(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob(filepath.Join("testdata", "repro", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no repro files under testdata/repro")
	}
	for _, p := range paths {
		p := p
		t.Run(filepath.Base(p), func(t *testing.T) {
			t.Parallel()
			old, err := ReadRepro(p)
			if err != nil {
				t.Fatal(err)
			}
			// the file itself is the compact encoding: one line, no
			// indentation, no derived readable field
			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(raw, []byte(`"readable"`)) || bytes.Contains(raw, []byte("\n ")) {
				t.Fatal("file on disk is not the compact encoding")
			}
			b, err := json.Marshal(old)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(b, []byte(`"readable"`)) {
				t.Fatal("compact encoding still carries readable")
			}
			if bytes.Contains(b, []byte("\n")) {
				t.Fatal("compact encoding is not one line")
			}
			var fresh Repro
			if err := json.Unmarshal(b, &fresh); err != nil {
				t.Fatal(err)
			}
			if fresh.Seed != old.Seed {
				t.Fatalf("seed moved: %d -> %d", old.Seed, fresh.Seed)
			}
			if !reproCardsEqual(old.Cards, fresh.Cards) {
				t.Fatal("cards moved")
			}
			if len(fresh.Actions) != len(old.Actions) {
				t.Fatalf("actions moved: %d -> %d", len(old.Actions), len(fresh.Actions))
			}
			for i := range old.Actions {
				if fresh.Actions[i] != old.Actions[i] {
					t.Fatalf("action %d moved: %s -> %s", i, old.Actions[i].String(), fresh.Actions[i].String())
				}
			}
		})
	}
}

func reproCardsEqual(a, b [2][]string) bool {
	for s := range a {
		if len(a[s]) != len(b[s]) {
			return false
		}
		for i := range a[s] {
			if a[s][i] != b[s][i] {
				return false
			}
		}
	}
	return true
}
