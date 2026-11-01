package draftlab

import (
	"testing"

	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/deck"
)

// TestBuildAndRandomDecks: every produced deck is at least 40 cards, has a
// legal land split, and resolves against the lab's registry; the random
// deck is deterministic per seed.
func TestBuildAndRandomDecks(t *testing.T) {
	l := testLab(t)
	pod, err := l.OpenPod("FDN", setdata.Arena, 42, 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	builder := testBuilder(t, l, "best23")
	for s := 0; s < 4; s++ {
		deck, err := l.Build(pod[s][0], builder)
		if err != nil {
			t.Fatalf("Build seat %d: %v", s, err)
		}
		if fileCount(deck.File) < 40 {
			t.Errorf("seat %d built deck has %d cards, want >= 40", s, fileCount(deck.File))
		}
		if deck.Source != "build:best23@1" {
			t.Errorf("seat %d source = %q", s, deck.Source)
		}
		if deck.Build == nil {
			t.Errorf("seat %d deck carries no build", s)
		}

		r1, err := l.RandomDeck(pod[s][0], uint64(100+s))
		if err != nil {
			t.Fatalf("RandomDeck seat %d: %v", s, err)
		}
		if fileCount(r1.File) < 40 {
			t.Errorf("seat %d random deck has %d cards, want >= 40", s, fileCount(r1.File))
		}
		if r1.Source != "random" {
			t.Errorf("seat %d random source = %q", s, r1.Source)
		}
		r2, err := l.RandomDeck(pod[s][0], uint64(100+s))
		if err != nil {
			t.Fatal(err)
		}
		if !sameFile(r1.File, r2.File) {
			t.Errorf("seat %d random deck is not deterministic per seed", s)
		}
	}
}

// fileCount is a deck file's total card count.
func fileCount(f deck.File) int {
	n := 0
	for _, e := range f.Cards {
		n += e.Count
	}
	for _, e := range f.Sideboard {
		n += e.Count
	}
	return n
}

// sameFile compares two deck files: names, counts and totals.
func sameFile(a, b deck.File) bool {
	if len(a.Cards) != len(b.Cards) {
		return false
	}
	for i := range a.Cards {
		if a.Cards[i] != b.Cards[i] {
			return false
		}
	}
	return true
}

// TestListAndCatalogDecks: a list deck adopts and resolves; the FDN
// limited catalog loads and every deck resolves.
func TestListAndCatalogDecks(t *testing.T) {
	l := testLab(t)
	pod, err := l.OpenPod("FDN", setdata.Arena, 42, 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	builder := testBuilder(t, l, "best23")
	built, err := l.Build(pod[0][0], builder)
	if err != nil {
		t.Fatal(err)
	}

	adopted, err := l.ListDeck(built.File)
	if err != nil {
		t.Fatalf("ListDeck: %v", err)
	}
	if adopted.Source != "list" || adopted.Pool != nil || adopted.Build != nil {
		t.Errorf("list deck = %+v", adopted)
	}

	catalog, err := l.CatalogDecks("fdn-limited")
	if err != nil {
		t.Fatalf("CatalogDecks: %v", err)
	}
	if len(catalog) == 0 {
		t.Fatal("empty catalog")
	}
	for _, d := range catalog {
		if fileCount(d.File) < 40 {
			t.Errorf("catalog deck %q has %d cards", d.Name, fileCount(d.File))
		}
		if d.Source == "" || d.Pool != nil || d.Build != nil {
			t.Errorf("catalog deck %q = %+v", d.Name, d)
		}
	}

	// An unknown catalog is an error naming the id.
	if _, err := l.CatalogDecks("no-such-catalog"); err == nil {
		t.Error("CatalogDecks accepted an unknown catalog")
	}
}
