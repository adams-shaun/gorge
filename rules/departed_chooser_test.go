package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// Task fx37: pin the event stream of the CR 800.4f departed-chooser path —
// releasePendingDecisionOfDepartedPlayer (rules/sba.go). A player leaves the
// game while one of their OWN decisions is outstanding; that decision can
// never be answered, so the engine clears e.pending and, if a resolution was
// suspended (e.resume != nil, a mid-resolution ask), resumes it with a nil
// answer so the asking instruction gets no selection and the rest of the
// effect chain — including CR 608.2n's completion tail — still runs.
//
// This engine's contract is byte-identical deterministic replay over a
// sha256 hash-chained event log, so a path that reaches the right game state
// by a different sequence (or number) of events is a divergence even when
// every board assertion passes. The departed-chooser path is exactly the
// shape where that hides: it runs a resolution tail with a nil answer, and
// nothing before this test asserted which events that tail emits. This test
// pins the event count, the kinds and order of the resumption tail, the
// resulting chain head as a constant, and that a log-only replay
// (replayFromLog, the rules-internal reconstruction that folds the recorded
// events straight through events.Apply — a state change that bypassed an
// event emit is invisible to it) rebuilds the identical Game.
//
// Fixture choice: a REAL corpus card — Chain Lightning — because its
// DealDamage->CopySpellAbility chain poses a genuine mid-resolution
// UnlessCost$ ask (the ResumeKind "unless_pay" shape), which is precisely
// the suspended-resolution arm of releasePendingDecisionOfDepartedPlayer.
// A synthetic fixture could manufacture the same ask, but Chain Lightning
// reaches it through the real cast/resolve path with no manufactured SAs,
// and its pay-or-copy oracle is the repo's own established
// departed-payer probe (see TestCR608CompletedSpellLeavesStackAfterDeparted-
// Payer). The departure is a concession (CR 104.3a): it is the only lawful
// way to leave while your own resolution is suspended on the stack, and it
// is emitted directly rather than through an intent — a concession is a
// priority option, but the suspended ask is NOT a priority decision, so no
// Submit can express it at this point, and a 3-seat game keeps playing on
// after the loss.
func TestDepartedChooserResumptionEventStreamIsDeterministic(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	cfg := Config{Seed: 42, Tokens: reg.Tokens}
	for _, n := range []string{"ur-delver", "death-n-taxes", "ur-delver"} {
		cfg.Names = append(cfg.Names, n)
		cfg.Decks = append(cfg.Decks, testutil.RepoDeck(t, reg, n))
	}
	e := New(cfg)
	e.Advance()
	toMain1(t, e)

	chain := crAbortMove(t, e, 0, "Chain Lightning", state.ZHand)
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -17})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 2})
	e.askPriority(0)
	crAbortAnswer(t, e, "Chain Lightning", crAbortOption(t, e, "Chain Lightning", "cast", chain))
	crResolutionPlayerTarget(t, e, 1)
	crResolutionRound(t, e)

	// We are standing exactly on the shape under test: a resolution
	// suspended on player 1's own UnlessCost$ paid-or-copy ask.
	d := e.Pending()
	if d == nil || d.Player != 1 || d.ResumeKind != "unless_pay" || !e.Suspended() || e.G.Players[1].Lost {
		t.Fatalf("portended state: want a suspended unless_pay ask at a living seat 1; "+
			"pending=%+v suspended=%v life=%d lost=%v", d, e.Suspended(), e.G.Players[1].Life, e.G.Players[1].Lost)
	}

	// The path under test: the departure itself, then the checkStateBased
	// call that runs the CR 800.4a sweep and the release hook.
	start := len(e.L.Events)
	e.emit(events.Event{Kind: events.PlayerLost, Player: 1, Text: "conceded"})
	e.checkStateBased()
	path := e.L.Events[start:]
	if len(path) != 64 {
		t.Fatalf("departed-chooser path emitted %d events, want 64 (events change => a divergence is hiding here)",
			len(path))
	}

	// Kinds and order. The departure sweep (CR 800.4a: the departed player's
	// owned objects cease to exist) is the move_zone "player left the game"
	// group; the resumption tail is the single chain completion MoveZone,
	// then the resumed priority round's Priority and DecisionAsk. Encode the
	// kind stream as run lengths so both the count and the order are pinned.
	runs := kindRuns(path)
	want := [][2]int{{int(events.PlayerLost), 1}, {int(events.MoveZone), 61},
		{int(events.Priority), 1}, {int(events.DecisionAsk), 1}}
	if len(runs) != len(want) {
		t.Fatalf("tail kind runs = %v, want %v", runs, want)
	}
	for i := range want {
		if runs[i] != want[i] {
			t.Fatalf("tail kind run[%d] = %v, want %v (full runs %v)", i, runs[i], want[i], runs)
		}
	}

	// CR 608.2n's completion tail: the suspended Chain Lightning leaves the
	// stack for the graveyard — the last MoveZone before the resumed
	// priority — and no Resolve re-fires (it fired once, before the ask).
	if got := path[len(path)-3]; got.Kind != events.MoveZone || got.Obj != chain ||
		got.From != state.ZStack || got.To != state.ZGraveyard {
		t.Fatalf("completion tail = %+v (seq %d), want MoveZone of the chain from stack to graveyard",
			got, got.Seq)
	}
	if got := path[len(path)-2]; got.Kind != events.Priority {
		t.Fatalf("event before last = %s (seq %d), want Priority resuming the round", got.Kind, got.Seq)
	}
	if got := path[len(path)-1]; got.Kind != events.DecisionAsk {
		t.Fatalf("last event = %s (seq %d), want DecisionAsk for the resumed round", got.Kind, got.Seq)
	}

	// The 61 MoveZone events: 60 are the departure sweep, and exactly one is
	// the chain's completion move. Everything else in the tail must be the
	// resumed-round preface (Priority, DecisionAsk) — no stray Resolve,
	// ModeChosen, Note, Damage or LifeChange from the abandoned continuation.
	sweeps, completions, other := 0, 0, 0
	for _, ev := range path {
		switch {
		case ev.Kind == events.MoveZone && ev.Text == "player left the game":
			sweeps++
		case ev.Kind == events.MoveZone && ev.Obj == chain:
			completions++
		case ev.Kind == events.Priority || ev.Kind == events.DecisionAsk:
			// the resumed round's preface — counted implicitly below.
		case ev.Kind == events.PlayerLost:
			// the departure record itself (the first event of the path).
		default:
			other++
		}
		if ev.Player == 1 {
			// The departure PlayerLost (the first event) is necessarily
			// attributed to seat 1 as the record of the loss; nothing the
			// resumption tail emits may be.
			if ev.Kind != events.PlayerLost {
				t.Fatalf("resumption event attributed to the departed player 1: seq %d %s", ev.Seq, ev.Kind)
			}
		}
	}
	if sweeps != 60 {
		t.Fatalf("departure sweep emitted %d 'player left the game' moves, want 60", sweeps)
	}
	if completions != 1 {
		t.Fatalf("chain completion move emitted %d times, want 1", completions)
	}
	if other != 0 {
		t.Fatalf("resumption tail emitted %d unexpected events", other)
	}
	if !e.G.Players[1].Lost || e.G.Over {
		t.Fatalf("concession must leave a three-seat game running: lost=%v over=%v", e.G.Players[1].Lost, e.G.Over)
	}
	if e.G.Obj(chain).Zone != state.ZGraveyard || len(e.G.Stack) != 0 {
		t.Fatalf("chain zone=%s stack=%v, want graveyard and an empty stack", e.G.Obj(chain).Zone, e.G.Stack)
	}

	// The chain head as a constant: a reordering that preserves counts would
	// still change it, so this is the assertion a count-only test cannot make.
	if got := e.L.Head(); got != "08a5b034057d2b7b" {
		t.Fatalf("chain head = %s, want 08a5b034057d2b7b", got)
	}

	// T21-e: a log-only replay must reconstruct the identical Game. If any
	// state change on this path bypassed an event emit (the release clears
	// e.resume and e.pending by hand, the natural place such a bypass would
	// hide), a reconstruction that folds only the logged events cannot know
	// about it and this comparison is what catches it.
	fresh := replayFromLog(t, cfg, e.L.Events)
	if diff := diffGames(e.G, fresh); diff != "" {
		t.Fatalf("log-only replay diverges:\n%s", diff)
	}
}

// kindRuns run-length encodes a slice of events as (kind, count) pairs.
func kindRuns(evs []events.Event) [][2]int {
	var runs [][2]int
	for _, ev := range evs {
		if n := len(runs); n > 0 && runs[n-1][0] == int(ev.Kind) {
			runs[n-1][1]++
			continue
		}
		runs = append(runs, [2]int{int(ev.Kind), 1})
	}
	return runs
}
