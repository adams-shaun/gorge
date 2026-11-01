package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRunPlaysAndVerifiesATwoSeatGame: run's io.Writer signature is exercised
// directly, with no subprocess and no real file, over a 2-seat, 1-game
// -verify run against the repo's own decks.
func TestRunPlaysAndVerifiesATwoSeatGame(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
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

// TestPrintReplayOutcomeHonoursMissingWithoutAZeroEvent: feed a hand-built *replay.Divergence with
// Missing: true through the printing branch directly (no need to corrupt a
// real event log to force one) and assert the wording names Seq and Got.Kind
// -- and, crucially, never prints the zero events.Event's Kind
// ("game_start") that a naive "always print Want" branch would produce for
// the Missing case, where Want is documented meaningless
// (engine/replay/replay.go's own Divergence doc).
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
