package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
)

// playSet is PlaySet for a test, skipping only when the checkout has no corpus.
func playSet(t *testing.T, name string) cardset.Set {
	t.Helper()
	set, err := oracle.PlaySet(name)
	if errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("set %s: %v", name, err)
	}
	if err != nil {
		t.Fatalf("set %s: %v", name, err)
	}
	return set
}

// TestDplayPlaysSyntheticPoolWithUnverifiedCards plays a pair of unverified cards
// from a synthetic pool, which ByName cannot see at all.
func TestDplayPlaysSyntheticPoolWithUnverifiedCards(t *testing.T) {
	name, cards := unverifiedSynthCards(t, 2)
	set := playSet(t, name)
	for _, card := range cards {
		i, ok := set.Lookup(card)
		if !ok {
			t.Fatalf("%s does not hold %q", name, card)
		}
		if sup := set.Supported(); !sup[i] {
			t.Fatalf("%q is unverified but not playable in %s", card, name)
		}
	}
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"-set", name, "-a", seatRandom, "-b", seatRandom, "-pairs", "1",
		"-deck-a", writeDeck(t, dir, "a", cards[0]), "-deck-b", writeDeck(t, dir, "b", cards[1])}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr: %s", code, stderr.String())
	}
	if rows := strings.Split(strings.TrimSpace(stdout.String()), "\n"); len(rows) != 1 || rows[0] == "" {
		t.Fatalf("games rows: %q", stdout.String())
	}
}

// unverifiedSynthCards is the first synthetic pool, by name, that holds at least n
// unverified deck cards, with the first n of them; it skips when the checkout has no
// corpus. Evidence rounds verify cards, so no test names a card it needs to stay
// unverified.
func unverifiedSynthCards(t *testing.T, n int) (string, []string) {
	t.Helper()
	names, err := cardset.SynthNames()
	if errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("synthetic pools: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if !strings.HasPrefix(name, "std-") {
			continue
		}
		set := playSet(t, name)
		var cards []string
		for i, card := range set.Names()[:set.NumCards()] {
			if set.Status(i) == cardset.StatusUnverified {
				cards = append(cards, card)
			}
		}
		if len(cards) >= n {
			return name, cards[:n]
		}
	}
	t.Fatalf("no synthetic pool holds %d unverified cards", n)
	return "", nil
}

// TestDplaySkipRefusedSkipsOffSetDeck holds a deck outside the pool to the refusal
// path under -skip-refused, and to the hard error without it.
func TestDplaySkipRefusedSkipsOffSetDeck(t *testing.T) {
	set := playSet(t, "std-trk")
	const off = "Loot, Exuberant Explorer"
	if _, ok := set.Lookup(off); ok {
		t.Fatalf("precondition: %q is in std-trk, so the test cannot exercise an off-set card", off)
	}
	dir := t.TempDir()
	writeDeck(t, dir, "good", "The Holodeck")
	writeDeck(t, dir, "off", off)
	pairs := filepath.Join(dir, "pairs.tsv")
	if err := os.WriteFile(pairs, []byte("good\tgood\t1\ngood\toff\t2\noff\tgood\t3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sumPath := filepath.Join(dir, "summary.json")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-set", "std-trk", "-a", seatRandom, "-b", seatRandom, "-skip-refused",
		"-decks", dir, "-pairs-file", pairs, "-summary", sumPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr: %s", code, stderr.String())
	}
	raw, err := os.ReadFile(sumPath)
	if err != nil {
		t.Fatal(err)
	}
	var s summary
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	if s.Games != 1 || s.RefusedPairs != 2 || s.RefusedCards[off] != 2 {
		t.Fatalf("games %d, refused pairs %d, %q in %d decks; want 1, 2 and 2", s.Games, s.RefusedPairs, off, s.RefusedCards[off])
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"-set", "std-trk", "-a", seatRandom, "-b", seatRandom,
		"-decks", dir, "-pairs-file", pairs, "-summary", sumPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("without -skip-refused: exit %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), off) {
		t.Fatalf("stderr does not name %q: %s", off, stderr.String())
	}
}
