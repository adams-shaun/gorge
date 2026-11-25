package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

const deckDir = "../../../testkit/testutil/decks"

func TestSeatingRotatesEveryPod(t *testing.T) {
	seen := map[[podSeats]int]bool{}
	for g := 0; g < podSeats*len(fdcDecks); g++ {
		s := seating(g)
		skip := (g / podSeats) % len(fdcDecks)
		for _, i := range s {
			if i == skip {
				t.Fatalf("game %d seats the left-out deck %d: %v", g, skip, s)
			}
		}
		seen[s] = true
	}
	if len(seen) != podSeats*len(fdcDecks) {
		t.Fatalf("%d distinct seatings in 20 games, want 20", len(seen))
	}
}

// TestPodBuildsAndPlays: every 4-deck pod of the substituted precons builds a
// table, the decks written for gorge are 100 cards with the commander kept,
// and a short run plays whole 4-seat games.
func TestPodBuildsAndPlays(t *testing.T) {
	dir, err := cardset.CorpusDir()
	if errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no corpus: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := buildPod(reg, deckDir)
	if err != nil {
		t.Fatal(err)
	}
	for skip, tbl := range p.tbls {
		if tbl == nil {
			t.Fatalf("pod without %s has no table", fdcDecks[skip])
		}
	}
	out := t.TempDir()
	if err := p.writeTo(out); err != nil {
		t.Fatal(err)
	}
	for _, name := range fdcDecks {
		f, err := readDeck(out, name)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, e := range f.Cards {
			n += e.Count
		}
		if n != 100 {
			t.Errorf("%s: %d cards, want 100", name, n)
		}
		if err := f.ValidateCommander(reg); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	tsv, err := os.ReadFile(filepath.Join(out, "substitutions.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(tsv), "deck\tcard\tcopies\treplacement\twhy\n") {
		t.Fatalf("substitutions.tsv header: %.80q", tsv)
	}
	var sum summary
	gm := &game{cfg: dirac.LockstepConfig(dirac.SurfaceAutopay)}
	gm.cfg.Commander = true
	for g := 0; g < 3; g++ {
		gm.playOne(p, 1, g, &sum)
	}
	if sum.Games != 3 || sum.Errors != 0 || sum.Decisions == 0 {
		t.Fatalf("3 games: %+v", sum)
	}
	t.Logf("%d card copies substituted; 3 games: %d turns, %d decisions, %d stalls", len(p.rows), sum.Turns, sum.Decisions, sum.Stalls)
}
