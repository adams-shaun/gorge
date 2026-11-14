package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
)

// TestReproReplaysACustomDecksPool: a repro of a game from a custom -decks pool
// replays under the same flags, though its decks hold a card of two faces. A
// repro names a card by its front face and the pool's table by "Front // Back",
// so the replay answered `card "Imodane's Recruiter" is not in the pool` for
// every pod that drew one (the draftsim lock pool of the four-seat MBC runs).
func TestReproReplaysACustomDecksPool(t *testing.T) {
	cardsDir, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	set, err := cardset.ByName("mbc")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no mbc table (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	const front, full = "Imodane's Recruiter", "Imodane's Recruiter // Train Troops"
	if _, ok := set.Lookup(front); ok {
		t.Fatalf("precondition: the mbc table names %q by its front face", front)
	}
	if _, ok := set.Lookup(full); !ok {
		t.Skipf("the mbc tables have no %q (missing from the pool)", full)
	}
	// the custom pool: one deck file under -decks, as the lock pool's decks are
	decks := t.TempDir()
	deckFile := map[string]any{"name": "lock_a", "format": "constructed", "cards": []map[string]any{
		{"name": full, "count": 4}, {"name": "Plains", "count": 18}, {"name": "Mountain", "count": 18},
	}}
	b, _ := json.Marshal(deckFile)
	if err := os.WriteFile(filepath.Join(decks, "lock_a.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	var list []string
	for range 4 {
		list = append(list, front)
	}
	for range 18 {
		list = append(list, "Plains", "Mountain")
	}
	rep := &oracle.Repro{Format: "koracle-repro-2", Decks: [2]string{"lock_a", "lock_b"}, Cards: [2][]string{list, list},
		Seed: 7, Surface: "full", Kernel: "set mbc", MaxTurns: 4}
	path := filepath.Join(t.TempDir(), "found.repro.json")
	if err := rep.Write(path); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runKFuzz(t, "-kernel", "mbc", "-decks", decks, "-cards", cardsDir, "-repro", path)
	if strings.Contains(stderr, "is not in the pool") {
		t.Fatalf("the replay lost the two-faced card: %s", stderr)
	}
	if code != 0 || !strings.Contains(stdout, "repro: no divergence") {
		t.Fatalf("exit %d, want 0 and a clean replay (stdout %q stderr %q)", code, stdout, stderr)
	}
}
