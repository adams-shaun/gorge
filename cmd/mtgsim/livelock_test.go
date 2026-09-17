package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/view"
)

// livelockSeat is a seat whose Decide panics with a *rules.LivelockError --
// the shape the engine's livelock watcher (rules/livelock.go) produces from
// inside Submit when a game's event stream provably never terminates. The
// harness's drive loop must convert that panic into the game's failure and
// the diagnostic, not hang on it and not crash on it. The variants cover the
// guardrails: a seat error must still print FAILED, and a panic that is NOT
// a LivelockError must re-panic (a bug is a bug either way).
type livelockSeat struct {
	lle  *rules.LivelockError
	err  error
	boom any
}

func (s livelockSeat) Decide(context.Context, view.View, decision.Decision) (decision.Intent, error) {
	if s.boom != nil {
		panic(s.boom)
	}
	if s.err != nil {
		return decision.Intent{}, s.err
	}
	panic(s.lle)
}

func livelockTestEngine(t *testing.T) *rules.Engine {
	t.Helper()
	dir := corpusDirOrSkip(t)
	reg, err := testutil.OpenCorpusRegistry(dir)
	if err != nil {
		t.Fatalf("corpus: %v", err)
	}
	all := testutil.LegacyDeckNames()
	names := all[:2]
	decks := make([][]*cards.Card, 2)
	for i := range decks {
		decks[i], err = testutil.LoadRepoDeck(reg, names[i])
		if err != nil {
			t.Fatalf("deck %s: %v", names[i], err)
		}
	}
	cfg := rules.Config{Seed: 0, Names: names, Decks: decks, Tokens: reg.Tokens}
	e := rules.New(cfg)
	e.Advance()
	return e
}

// TestDriveGameRecoversLivelockAsError pins the harness half of the
// contract: a game the engine aborts with a *rules.LivelockError surfaces
// as that error from driveGame -- a marked failure, not a hang and not a
// panic -- so playOne can report it and move to the next game.
func TestDriveGameRecoversLivelockAsError(t *testing.T) {
	e := livelockTestEngine(t)
	want := &rules.LivelockError{Reason: "repeating cycle", CycleLen: 13, Repeats: 31}
	n, err := driveGame(e, livelockSeat{lle: want})
	if err == nil {
		t.Fatal("driveGame returned no error for a livelocked game")
	}
	got, ok := err.(*rules.LivelockError)
	if !ok {
		t.Fatalf("driveGame error is %T (%v), want *rules.LivelockError", err, err)
	}
	if got != want {
		t.Errorf("recovered diagnostic = %+v, want the watcher's own %+v", got, want)
	}
	if n != 0 {
		t.Errorf("intents = %d, want 0 (the seat aborts on the first decision)", n)
	}
}

// TestDriveGameRepanicsOtherPanics: the recover is scoped to the watcher's
// abort. Any other panic -- a real bug either way -- crashes loudly rather
// than being laundered into a game failure.
func TestDriveGameRepanicsOtherPanics(t *testing.T) {
	e := livelockTestEngine(t)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("driveGame swallowed a non-livelock panic")
		} else if r != "boom" {
			t.Fatalf("recovered %v, want the original panic value", r)
		}
	}()
	_, _ = driveGame(e, livelockSeat{boom: "boom"})
}

// TestReportDriveFailureMarksTheLivelock pins the output contract an agent
// parses: the LIVELOCK marker with the diagnostic on the failure line, and
// FAILED (not LIVELOCK) for an ordinary drive error.
func TestReportDriveFailureMarksTheLivelock(t *testing.T) {
	var buf bytes.Buffer
	lle := &rules.LivelockError{Reason: "repeating cycle", Object: 42, Repeats: 31, CycleLen: 13}
	if reportDriveFailure(&buf, 7, 2, 12, lle) {
		t.Fatal("reportDriveFailure must signal failure")
	}
	out := buf.String()
	if !strings.Contains(out, "LIVELOCK") || !strings.Contains(out, "repeated 31 time(s)") || !strings.Contains(out, "seed 7") {
		t.Errorf("livelock line = %q, want the LIVELOCK marker, the diagnostic and the seed", out)
	}
	buf.Reset()
	if reportDriveFailure(&buf, 7, 2, 12, context.DeadlineExceeded) {
		t.Fatal("reportDriveFailure must signal failure")
	}
	if !strings.Contains(buf.String(), "FAILED") || strings.Contains(buf.String(), "LIVELOCK") {
		t.Errorf("ordinary failure line = %q, want FAILED without the LIVELOCK marker", buf.String())
	}
}
