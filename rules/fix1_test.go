package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// smallDeckGame builds an n-player engine with decks just barely bigger than
// the opening hand, so the players deck out (and the game reaches Over)
// after a handful of turns instead of the ~30+ turns a 40-card deck needs.
// Returns the Config used, since a fresh replay needs it (Ruling D).
func smallDeckGame(t *testing.T, n, deckSize int) (*Engine, Config) {
	t.Helper()
	names := make([]string, n)
	decks := make([][]*cards.Card, n)
	for i := range names {
		names[i] = string(rune('a' + i))
		decks[i] = mountainDeck(t, deckSize)
	}
	cfg := Config{Seed: 7, Names: names, Decks: decks}
	e := New(cfg)
	e.Advance()
	return e, cfg
}

// driveToOver repeatedly answers "pass" until the game ends or limit
// iterations elapse. Fails the test if the game never ends.
func driveToOver(t *testing.T, e *Engine, limit int) {
	t.Helper()
	for i := 0; i < limit && !e.G.Over; i++ {
		if passAll(t, e, 1) == 0 {
			break
		}
	}
	if !e.G.Over {
		t.Fatalf("game did not reach Over within %d pass rounds", limit)
	}
}

// TestReplayReconstructsPassesAndPriority is the Ruling A regression test:
// Passes and Priority must flow through events, not direct field writes, so
// that replaying the log alone (no rules-package code involved) reproduces
// them. If a write to G.Passes or G.Priority ever bypasses an emit again,
// the live value will drift from what the log can reconstruct and this test
// fails.
func TestReplayReconstructsPassesAndPriority(t *testing.T) {
	e, cfg := smallDeckGame(t, 2, 8)
	driveToOver(t, e, 5000)

	// Replay through events.Apply alone -- no Engine, no rules code. Genesis
	// (state.NewGame plus the deck-building AddObject calls in Engine.New)
	// is not itself logged (Ruling D), so it is not replayed here either;
	// Passes and Priority never depend on the object arena being populated.
	fresh := state.NewGame(cfg.Names)
	for _, ev := range e.L.Events {
		events.Apply(fresh, ev)
	}

	if fresh.Passes != e.G.Passes {
		t.Errorf("replayed Passes = %d, want %d (live)", fresh.Passes, e.G.Passes)
	}
	if fresh.Priority != e.G.Priority {
		t.Errorf("replayed Priority = %d, want %d (live)", fresh.Priority, e.G.Priority)
	}
}

// TestFinishedGameRejectsSubmit is the first Ruling B regression test: once
// a game ends, no further decision should be outstanding at all.
func TestFinishedGameRejectsSubmit(t *testing.T) {
	e, _ := smallDeckGame(t, 2, 8)
	driveToOver(t, e, 5000)

	if d := e.Pending(); d != nil {
		t.Fatalf("Pending() returned a live decision after Over: %+v", d)
	}
	if err := e.Submit(decision.Intent{Player: 0, Choices: []int{0}}); err == nil {
		t.Fatalf("Submit succeeded on a finished game")
	}
}

// TestSubmitGuardsOverEvenWithStalePending is the second Ruling B regression
// test. It reproduces the reviewer's exact failure mode directly: a live
// decision left queued (however that happens) after e.G.Over is set. Submit
// must refuse it regardless of what Pending() currently holds; this is a
// second, independent gate from the priorityRound fix above.
func TestSubmitGuardsOverEvenWithStalePending(t *testing.T) {
	e := newSeats(t, 2)
	e.emit(events.Event{Kind: events.GameOver, Player: 0})
	stale := &decision.Decision{
		Player: 1, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "pass", Label: "Pass priority"}},
	}
	e.pending = stale

	err := e.Submit(decision.Intent{Seq: stale.Seq, Player: 1, Choices: []int{0}})
	if err == nil {
		t.Fatalf("Submit succeeded on a finished game with a stale pending decision")
	}
	if e.pending != stale {
		t.Fatalf("Submit consumed the stale pending decision instead of rejecting it outright")
	}
}

// TestDrawStepSkipsEliminatedActivePlayer is the Ruling C regression test:
// the once-per-turn draw must check that the active player is still alive
// before drawing for them.
func TestDrawStepSkipsEliminatedActivePlayer(t *testing.T) {
	e := newSeats(t, 4)
	active := e.G.Active

	// Force exactly the state priorityRound's draw guard inspects: turn 2 (so
	// Turn > 1), draw step, no passes yet, priority already on the active
	// seat -- and that seat eliminated.
	e.emit(events.Event{Kind: events.TurnChange, Player: active, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDraw})
	e.emit(events.Event{Kind: events.Priority, Player: active})
	e.emit(events.Event{Kind: events.PlayerLost, Player: active})

	libBefore := len(e.G.Zone(state.ZLibrary, active))
	firstNewEvent := len(e.L.Events)

	e.priorityRound()

	for _, ev := range e.L.Events[firstNewEvent:] {
		if ev.Kind == events.Draw && ev.Player == active {
			t.Fatalf("draw event emitted for eliminated active player %d", active)
		}
	}
	if got := len(e.G.Zone(state.ZLibrary, active)); got != libBefore {
		t.Fatalf("library size for eliminated active player changed: %d -> %d", libBefore, got)
	}
}
