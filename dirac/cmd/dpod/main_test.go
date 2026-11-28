package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
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
	reg, p := testPod(t)
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

var (
	podOnce sync.Once
	podReg  *cards.Registry
	podP    *pod
	podErr  error
)

// testPod is the substituted pod, built once per test binary.
func testPod(t *testing.T) (*cards.Registry, *pod) {
	t.Helper()
	dir, err := cardset.CorpusDir()
	if errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no corpus: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	podOnce.Do(func() {
		if podReg, podErr = cards.SharedCorpus(dir); podErr == nil {
			podP, podErr = buildPod(podReg, deckDir)
		}
	})
	if podErr != nil {
		t.Fatal(podErr)
	}
	return podReg, podP
}

// TestPodHBotIsDeterministic: -policy hbot seats the heuristic bot on all
// four seats; the same seed plays the same games twice, and the bot finishes
// games (a win, not a cap or a stall).
func TestPodHBotIsDeterministic(t *testing.T) {
	reg, p := testPod(t)
	const games = 3
	var sums [2]summary
	for i := range sums {
		gm := &game{cfg: dirac.LockstepConfig(dirac.SurfaceAutopay)}
		gm.cfg.Commander = true
		gm.useHBot(reg, p)
		for g := 0; g < games; g++ {
			gm.playOne(p, 7, g, &sums[i])
		}
	}
	a, b := sums[0], sums[1]
	a.Substitute, b.Substitute = nil, nil
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("same seed, two runs:\n%+v\n%+v", a, b)
	}
	if a.Games != games || a.Errors != 0 || a.Decisions == 0 {
		t.Fatalf("%d games: %+v", games, a)
	}
	if a.Stalls == games {
		t.Fatalf("no hbot game finished: %+v", a)
	}
	t.Logf("hbot, %d games: %d turns, %d decisions, %d stalls %v, wins %v", games, a.Turns, a.Decisions, a.Stalls, a.StallKinds, a.Wins)
}

// TestPodQuietProofHBot: the heuristic bot on every seat of the auto-pay
// pods, with the quiet-seat proof's verify on (dirac/quiet_seat.go): every
// posed priority walk runs, and a proved window whose walk offers more than
// pass, or changes the State, fails the game (playOne counts its panic).
func TestPodQuietProofHBot(t *testing.T) {
	reg, p := testPod(t)
	for _, tb := range p.tbls {
		dirac.Register(tb)
	}
	st := &dirac.QuietStats{Panic: true}
	dirac.DebugQuietVerify = st
	defer func() { dirac.DebugQuietVerify = nil }()
	gm := &game{cfg: dirac.LockstepConfig(dirac.SurfaceAutopay)}
	gm.cfg.Commander = true
	gm.useHBot(reg, p)
	var sum summary
	for g := 0; sum.Decisions < 60000; g++ {
		gm.playOne(p, 1, g, &sum)
	}
	if st.First != "" {
		t.Fatal(st.First)
	}
	if sum.Errors != 0 || st.Proved == 0 {
		t.Fatalf("%d games, %d errors, %d of %d walks proved", sum.Games, sum.Errors, st.Proved, st.Walks)
	}
	t.Logf("%d games, %d decisions: %d walks, %d pass-only, %d proved", sum.Games, sum.Decisions, st.Walks, st.PassOnly, st.Proved)
}
