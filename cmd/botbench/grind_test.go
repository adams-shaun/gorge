package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/internal/testutil"
)

// TestGrindBudgetsIsTheOneGrindEngineTest pins, in one pass over real deck
// games (kept short with a max-turns cap; every assertion is seed-pure):
//
//   - the iteration budget is exact: a deck ground with iters=2 plays exactly
//     2 games, and two identical calls report identical tallies;
//   - iteration i is the same GAME at every run length (grindSeed's
//     prefix-stability property the report's reproducibility rests on): the
//     first k outcomes of a 2-iteration grind equal the first k of a
//     3-iteration grind at the same base;
//   - different deck indexes play different games (the stride keeps the
//     streams apart);
//   - an already-expired wall-clock budget still plays its FIRST game (the
//     deadline is checked between games) and only that game.
func TestGrindBudgetsIsTheOneGrindEngineTest(t *testing.T) {
	reg := benchCorpus(t)
	deck, err := testutil.LoadRepoDeck(reg, "death-n-taxes")
	if err != nil {
		t.Fatal(err)
	}
	run1, err := grindOne(3, 0, "death-n-taxes", deck, []int{}, false, time.Time{}, 2, 15, 20000, nil, nil, nil)
	if err != nil {
		t.Fatalf("grindOne: %v", err)
	}
	if run1.iters != 2 {
		t.Errorf("iters = %d, want exactly 2", run1.iters)
	}
	if run1.stalls > run1.iters {
		t.Errorf("stalls = %d exceeds iters = %d", run1.stalls, run1.iters)
	}
	run2, err := grindOne(3, 0, "death-n-taxes", deck, []int{}, false, time.Time{}, 2, 15, 20000, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if run2.intents != run1.intents || run2.turns != run1.turns || run2.wins != run1.wins {
		t.Errorf("two identical grinds diverged: %+v vs %+v", run1, run2)
	}

	long, err := grindOne(3, 0, "death-n-taxes", deck, []int{}, false, time.Time{}, 3, 15, 20000, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(long.outcomes) != 3 {
		t.Fatalf("long run played %d games, want 3", len(long.outcomes))
	}
	for i, o := range run1.outcomes {
		if o.intents != long.outcomes[i].intents || o.turns != long.outcomes[i].turns || o.winnerSeat != long.outcomes[i].winnerSeat {
			t.Errorf("iteration %d of the 2-game grind (intents %d, turns %d) differs from the 3-game grind's (%d, %d) -- grindSeed is not prefix-stable",
				i, o.intents, o.turns, long.outcomes[i].intents, long.outcomes[i].turns)
		}
	}

	// Deck indexes must stream apart.
	if grindSeed(0, 0, 0) == grindSeed(0, 1, 0) {
		t.Error("deck 0 and deck 1 play the same seed -- the stride collapsed")
	}
	if grindSeed(0, 1, 0) == grindSeed(0, 1, 1) {
		t.Error("iterations of one deck share a seed")
	}

	// An expired wall-clock budget: the between-games check guarantees at
	// least the first game, and only the first (it is the only unguarded
	// one).
	past := time.Now().Add(-time.Second)
	expired, err := grindOne(9, 0, "death-n-taxes", deck, []int{}, false, past, 0, 15, 20000, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if expired.iters != 1 {
		t.Errorf("an expired wall-clock budget played %d games, want exactly the one unguarded first game", expired.iters)
	}
}

// TestGrindAllDecksEndToEnd runs the real flag path over the full pool (one
// goroutine per deck) with a tight turn cap so the games stay short, and
// checks the report: the header, one line per deck at exactly the budgeted
// iteration count, and the combined line.
func TestGrindAllDecksEndToEnd(t *testing.T) {
	dir := corpusDirOrSkip(t)
	var out, prog bytes.Buffer
	if err := runGrind(11, "all", 0, 1, dir, "constructed", 8, 20000, &out, &prog); err != nil {
		t.Fatalf("runGrind: %v", err)
	}
	report := out.String()
	if !strings.HasPrefix(report, "grind: base seed 11, budget 1 iteration(s)/deck, ") {
		t.Errorf("missing grind header, got:\n%s", report)
	}
	for _, name := range testutil.RepoDeckNames() {
		if !grindRowSays(report, name, "1") {
			t.Errorf("deck %s must appear with exactly 1 iteration:\n%s", name, report)
		}
	}
	combinedOK := false
	wantIters := len(testutil.RepoDeckNames())
	for _, line := range strings.Split(report, "\n") {
		// The tabwriter pads the first column with spaces, so the combined
		// row is "combined<spaces>N<spaces><rate>"; match the row by its
		// line prefix and the total iteration count. N is the live pool
		// size, not a hardcoded 21: the pool grows when a deck is imported
		// (it was 21 before the Rakdos Muscle deck landed), and a count
		// pinned in the test would go stale on every import.
		if strings.HasPrefix(line, "combined") && strings.Contains(line, fmt.Sprintf(" %d ", wantIters)) {
			combinedOK = true
		}
	}
	if !combinedOK {
		t.Errorf("missing combined line (%d iterations over %d decks):\n%s", wantIters, wantIters, report)
	}
}

// grindRowSays reports whether the report's table carries a row whose first
// field is deck and whose second field (the iters column) is iters. The
// tabwriter pads columns with spaces, so the row is matched field-wise.
func grindRowSays(report, deck, iters string) bool {
	for _, line := range strings.Split(report, "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[0] == deck && f[1] == iters {
			return true
		}
	}
	return false
}

// TestGrindValidation pins the two flag validations that need no games: an
// unknown deck is refused, and -grind alongside -pairs is refused by
// mainExit before any corpus work.
func TestGrindValidation(t *testing.T) {
	dir := corpusDirOrSkip(t)
	var out, prog bytes.Buffer
	if err := runGrind(0, "no-such-deck", 0, 1, dir, "constructed", 0, 0, &out, &prog); err == nil {
		t.Error("an unknown deck must be an error")
	}
	code := mainExit("bot", "bot", 1, 0, 2, 0, "a:b", "constructed", "text", 0, 200, 20000, ".cards", false, false, "tron", 0, 0, "", "")
	if code == 0 {
		t.Error("-grind with -pairs must exit non-zero")
	}
}
