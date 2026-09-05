package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/replay"
)

// corpusDirOrSkip resolves the repo root the way testutil.CorpusRegistry and
// cards/boundary_test.go do -- git rev-parse --show-toplevel, never a
// hard-coded relative path -- and Skips when .cards/ is absent, so this test
// still passes on a clean checkout with nothing fetched. cmd/mtgsim cannot
// call testutil.CorpusRegistry directly for this: that helper returns a
// *cards.Registry, but run's -dir parameter wants a directory path, so this
// mirrors its resolve-and-skip shape instead of its return value.
func corpusDirOrSkip(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "-C", ".", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("could not resolve git repo root: %v", err)
	}
	dir := filepath.Join(strings.TrimSpace(string(out)), ".cards")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("no .cards/ corpus present -- run `make fetch-cards compile-cards`")
	}
	return dir
}

// TestRunPlaysAndVerifiesATwoSeatGame is the fix-round-1 test Minor #3 asked
// for: run's io.Writer signature (rather than the old *os.File) is exercised
// directly, with no subprocess and no real file, over a 2-seat, 1-game
// -verify run against the repo's own decks.
func TestRunPlaysAndVerifiesATwoSeatGame(t *testing.T) {
	dir := corpusDirOrSkip(t)
	var buf bytes.Buffer
	if err := run("", 2, 0, 1, true, false, dir, &buf); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "2 seats,") {
		t.Errorf("output missing the seat count: %q", out)
	}
	if !strings.Contains(out, "intents,") || !strings.Contains(out, "events,") || !strings.Contains(out, "turns,") {
		t.Errorf("output missing the per-game line shape (intents/events/turns): %q", out)
	}
	if !strings.Contains(out, "winner=") && !strings.Contains(out, "draw") {
		t.Errorf("output missing winner=... or draw: %q", out)
	}
	if !strings.Contains(out, "replay OK") {
		t.Errorf("output missing \"replay OK\": %q", out)
	}
}

// TestPrintReplayOutcomeHonoursMissingWithoutAZeroEvent is the fix-round-1
// test Minor #3 asked for: feed a hand-built *replay.Divergence with
// Missing: true through the printing branch directly (no need to corrupt a
// real event log to force one) and assert the wording names Seq and Got.Kind
// -- and, crucially, never prints the zero events.Event's Kind
// ("game_start") that a naive "always print Want" branch would produce for
// the Missing case, where Want is documented meaningless
// (replay/replay.go's own Divergence doc).
func TestPrintReplayOutcomeHonoursMissingWithoutAZeroEvent(t *testing.T) {
	var buf bytes.Buffer
	div := &replay.Divergence{Seq: 7, Missing: true, Got: events.Event{Kind: events.Draw}}
	ok := printReplayOutcome(&buf, div, "unused")
	if ok {
		t.Error("printReplayOutcome reported success for a divergence")
	}
	out := buf.String()
	if !strings.Contains(out, "event 7") {
		t.Errorf("output = %q, want it to name Seq 7", out)
	}
	if !strings.Contains(out, events.Draw.String()) {
		t.Errorf("output = %q, want it to name Got.Kind (%s)", out, events.Draw)
	}
	if strings.Contains(out, events.GameStart.String()) {
		t.Errorf("output = %q, printed the zero Event's Kind (%s) despite Missing -- Want should never be read here",
			out, events.GameStart)
	}
}
