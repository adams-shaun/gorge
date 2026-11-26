package kgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFDCGapsPoolIsCommitted pins the inputs of the fdc-gaps evidence row:
// scripts/dirac-evidence.sh plays the pool from these decks, so the row in
// evidence-games.tsv can be re-run. Every deck is 50 cards, holds the five
// Foundations Commander cards, and has a pool.tsv row.
func TestFDCGapsPoolIsCommitted(t *testing.T) {
	dir := filepath.Join(testRoot, "dirac/support/pools/fdc-gaps")
	files, err := filepath.Glob(filepath.Join(dir, "decks", "*.json"))
	if err != nil || len(files) != 4 {
		t.Fatalf("decks: %v, err %v; want 4", files, err)
	}
	tsv, err := os.ReadFile(filepath.Join(dir, "pool.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	five := []string{"Chandra's Ignition", "Fateful Absence", "Endless Atlas", "Padeem, Consul of Innovation", "Haven of the Spirit Dragon"}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var d struct {
			Name  string
			Cards []struct {
				Name  string
				Count int
			}
		}
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		n, have := 0, map[string]bool{}
		for _, c := range d.Cards {
			n += c.Count
			have[c.Name] = true
		}
		if n != 50 {
			t.Errorf("%s: %d cards, want 50", d.Name, n)
		}
		for _, c := range five {
			if !have[c] {
				t.Errorf("%s: lacks %s", d.Name, c)
			}
		}
		if !strings.Contains(string(tsv), d.Name+"\t") {
			t.Errorf("pool.tsv has no row for %s", d.Name)
		}
	}
	games, err := os.ReadFile(filepath.Join(testRoot, "dirac/support/evidence-games.tsv"))
	if err != nil || !strings.Contains(string(games), "\nfdc-gaps\t1500\n") {
		t.Errorf("evidence-games.tsv has no fdc-gaps row (err %v)", err)
	}
}
