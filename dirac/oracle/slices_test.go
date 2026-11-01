package oracle

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestSliceMatchesPool: a committed slice holds the decks its pairs name and, where
// the whole pool is on disk, those pairs are PairingAt's at SliceSeed and the decks
// are the pool's own bytes, so the slice plays the pool's first games.
func TestSliceMatchesPool(t *testing.T) {
	for _, want := range Pools() {
		t.Run(want.Name, func(t *testing.T) {
			dir, remove, err := ExtractSlice(want.Name)
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
			decks, err := filepath.Glob(filepath.Join(dir, "decks", "*.json"))
			if err != nil {
				t.Fatal(err)
			}
			have := make(map[string]bool, len(decks))
			for _, d := range decks {
				have[filepath.Base(d)] = true
			}
			for i, l := range lines {
				if !have[l.A+".json"] || !have[l.B+".json"] {
					t.Fatalf("pair %d: %s or %s is not in the slice", i, l.A, l.B)
				}
			}
			sub, err := ReadPool(filepath.Join(dir, "pool.tsv"), "all")
			if err != nil {
				t.Fatal(err)
			}
			if len(sub) != len(decks) {
				t.Errorf("pool.tsv lists %d decks, the slice holds %d", len(sub), len(decks))
			}

			p, err := PoolByName(want.Name)
			if err != nil {
				t.Fatal(err)
			}
			pool, err := ReadPool(filepath.Join(p.Inputs, "pool.tsv"), "all")
			if err != nil {
				t.Skipf("no whole pool to compare with (set GORGE_POOLS): %v", err)
			}
			for i, l := range lines {
				got := PairingAt(pool, i, SliceSeed)
				seed, err := PairSeed(l, i, SliceSeed)
				if err != nil {
					t.Fatal(err)
				}
				if got.A != l.A || got.B != l.B || got.Seed != seed {
					t.Fatalf("pair %d: slice %s vs %s seed %d, pool %s vs %s seed %d", i, l.A, l.B, seed, got.A, got.B, got.Seed)
				}
			}
			for _, d := range decks {
				a, err := os.ReadFile(d)
				if err != nil {
					t.Fatal(err)
				}
				b, err := os.ReadFile(filepath.Join(p.Inputs, "decks", filepath.Base(d)))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(a, b) {
					t.Errorf("%s differs from the pool's", filepath.Base(d))
				}
			}
		})
	}
}
