package event_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/draft"
)

// attemptStart logs the first GameStarted attempt of series (a, b) on table
// t with seed seed.
func attemptStart(t *testing.T, e *event.Event, round, a, b, attempt int, table string, seed uint64) {
	t.Helper()
	mustApply(t, e, event.Entry{
		Kind: event.KindGameStarted, Round: round, A: a, B: b,
		Game: 1, Attempt: attempt, Table: table, GameSeed: seed,
	})
}

// TestGameAttemptLifecycle walks one series through an aborted attempt and
// a successful retry, checking InFlight and NextAttempt at each step.
func TestGameAttemptLifecycle(t *testing.T) {
	e := newScriptedEventToRound1(t)
	if got := e.NextAttempt(0, 3); got != 1 {
		t.Fatalf("NextAttempt before any attempt = %d, want 1", got)
	}
	if e.Started(0, 3) {
		t.Fatal("Started before SeriesStarted = true")
	}
	mustApply(t, e, event.Entry{Kind: event.KindSeriesStarted, Round: 1, A: 0, B: 3, BestOf: 1})
	if !e.Started(0, 3) {
		t.Fatal("Started after SeriesStarted = false")
	}

	attemptStart(t, e, 1, 0, 3, 1, "t-abc", 99)
	in := e.InFlight()
	if len(in) != 1 || in[0] != (event.GameAttempt{Round: 1, A: 0, B: 3, Game: 1, Attempt: 1, Table: "t-abc", Seed: 99, Chooser: -1}) {
		t.Fatalf("InFlight after GameStarted = %+v", in)
	}
	if got := e.NextAttempt(0, 3); got != 2 {
		t.Fatalf("NextAttempt while attempt 1 in flight = %d, want 2", got)
	}
	// The other round-1 series has nothing in flight.
	if got := e.InFlight(); len(got) != 1 {
		t.Fatalf("InFlight reports %d attempts, want 1", len(got))
	}

	mustApply(t, e, event.Entry{Kind: event.KindGameAborted, Round: 1, A: 0, B: 3, Game: 1, Attempt: 1, Reason: "table crashed"})
	if got := e.InFlight(); len(got) != 0 {
		t.Fatalf("InFlight after GameAborted = %+v, want none", got)
	}
	if got := e.NextAttempt(0, 3); got != 2 {
		t.Fatalf("NextAttempt after abort = %d, want 2 (attempts kept)", got)
	}

	attemptStart(t, e, 1, 0, 3, 2, "t-abc", 104)
	if got := e.NextAttempt(0, 3); got != 3 {
		t.Fatalf("NextAttempt while attempt 2 in flight = %d, want 3", got)
	}
	w := 0
	mustApply(t, e, event.Entry{Kind: event.KindGameEnded, Round: 1, A: 0, B: 3, Game: 1, Attempt: 2, Winner: &w, Head: "h-r1-03"})
	if got := e.InFlight(); len(got) != 0 {
		t.Fatalf("InFlight after GameEnded = %+v, want none", got)
	}
	if got := e.NextAttempt(0, 3); got != 1 {
		t.Fatalf("NextAttempt after the game ended = %d, want 1 (attempts reset)", got)
	}
	mustApply(t, e, event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: 0, B: 3, Score: [2]int{1, 0}})

	// The next series of the same round starts its attempts from 1 again.
	mustApply(t, e, event.Entry{Kind: event.KindSeriesStarted, Round: 1, A: 1, B: 2, BestOf: 1})
	if got := e.NextAttempt(1, 2); got != 1 {
		t.Fatalf("NextAttempt for a fresh series = %d, want 1", got)
	}
}

// TestGameAttemptRejections checks each illegal attempt entry is rejected
// with the log unchanged.
func TestGameAttemptRejections(t *testing.T) {
	e := newScriptedEventToRound1(t)
	mustApply(t, e, event.Entry{Kind: event.KindSeriesStarted, Round: 1, A: 0, B: 3, BestOf: 1})
	attemptStart(t, e, 1, 0, 3, 1, "t-abc", 7)
	frozen := len(e.Log())

	w := 0
	rejects := []struct {
		name string
		en   event.Entry
	}{
		{"GameStarted while an attempt is in flight", event.Entry{Kind: event.KindGameStarted, Round: 1, A: 0, B: 3, Game: 1, Attempt: 2, Table: "t-abc"}},
		{"GameStarted with a wrong attempt", event.Entry{Kind: event.KindGameStarted, Round: 1, A: 0, B: 3, Game: 1, Attempt: 5, Table: "t-abc"}},
		{"GameStarted with an empty table", event.Entry{Kind: event.KindGameStarted, Round: 1, A: 0, B: 3, Game: 1, Attempt: 2}},
		{"GameStarted with the wrong game", event.Entry{Kind: event.KindGameStarted, Round: 1, A: 0, B: 3, Game: 2, Attempt: 2, Table: "t-abc"}},
		{"GameAborted with an empty reason", event.Entry{Kind: event.KindGameAborted, Round: 1, A: 0, B: 3, Game: 1, Attempt: 1}},
		{"GameAborted with the wrong attempt", event.Entry{Kind: event.KindGameAborted, Round: 1, A: 0, B: 3, Game: 1, Attempt: 2, Reason: "x"}},
		{"GameAborted with the wrong game", event.Entry{Kind: event.KindGameAborted, Round: 1, A: 0, B: 3, Game: 2, Attempt: 1, Reason: "x"}},
		{"GameEnded naming another game while one is in flight", event.Entry{Kind: event.KindGameEnded, Round: 1, A: 0, B: 3, Game: 2, Winner: &w, Head: "h"}},
		{"SeriesEnded while an attempt is in flight", event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: 0, B: 3, Score: [2]int{1, 0}}},
	}
	for _, c := range rejects {
		if err := e.Apply(c.en); err == nil {
			t.Errorf("%s: want error", c.name)
		}
		if got := len(e.Log()); got != frozen {
			t.Errorf("%s: log length changed from %d to %d", c.name, frozen, got)
		}
	}
	if got := e.InFlight(); len(got) != 1 || got[0].Attempt != 1 {
		t.Fatalf("InFlight after rejections = %+v, want the original attempt 1", got)
	}

	// Abort, then GameAborted with nothing in flight is rejected.
	mustApply(t, e, event.Entry{Kind: event.KindGameAborted, Round: 1, A: 0, B: 3, Game: 1, Attempt: 1, Reason: "host restart"})
	if err := e.Apply(event.Entry{Kind: event.KindGameAborted, Round: 1, A: 0, B: 3, Game: 1, Attempt: 1, Reason: "again"}); err == nil {
		t.Errorf("GameAborted with nothing in flight: want error")
	}

	// A GameStarted for a series that was never started is rejected, and
	// does not create series state for the non-pairing or unstarted pair.
	if err := e.Apply(event.Entry{Kind: event.KindGameStarted, Round: 1, A: 1, B: 2, Game: 1, Attempt: 1, Table: "t"}); err == nil {
		t.Errorf("GameStarted for an unstarted series: want error")
	}
	if err := e.Apply(event.Entry{Kind: event.KindGameStarted, Round: 1, A: 0, B: 1, Game: 1, Attempt: 1, Table: "t"}); err == nil {
		t.Errorf("GameStarted for a non-pairing: want error")
	}
	if e.Started(1, 2) || e.Started(0, 1) {
		t.Errorf("a rejected GameStarted created series state: Started(1,2)=%v Started(0,1)=%v", e.Started(1, 2), e.Started(0, 1))
	}
	if got := e.NextAttempt(1, 2); got != 1 {
		t.Errorf("NextAttempt for an unstarted series = %d, want 1", got)
	}
}

// TestLogWithoutGameStartedShape pins that a log with no GameStarted marshals
// without the attempt, table, game seed or reason fields.
func TestLogWithoutGameStartedShape(t *testing.T) {
	e := newScriptedEvent(t)
	if !e.Done() {
		t.Fatalf("scripted event ended in phase %s, want COMPLETE", e.Phase())
	}
	wire, err := event.MarshalLog(e.Log())
	if err != nil {
		t.Fatalf("MarshalLog: %v", err)
	}
	for _, frag := range []string{`"attempt"`, `"table"`, `"game_seed"`, `"reason"`} {
		if strings.Contains(string(wire), frag) {
			t.Errorf("log without GameStarted contains %s", frag)
		}
	}
}

// TestAttemptLogReplaysExactly round-trips a log that carries attempts
// through MarshalLog/ReplayLog to an equal MarshalLog and StateJSON.
func TestAttemptLogReplaysExactly(t *testing.T) {
	e := newScriptedEventToRound1(t)
	mustApply(t, e, event.Entry{Kind: event.KindSeriesStarted, Round: 1, A: 0, B: 3, BestOf: 1})
	attemptStart(t, e, 1, 0, 3, 1, "t-abc", 7)
	mustApply(t, e, event.Entry{Kind: event.KindGameAborted, Round: 1, A: 0, B: 3, Game: 1, Attempt: 1, Reason: "crash"})
	attemptStart(t, e, 1, 0, 3, 2, "t-abc", 104)
	w := 0
	mustApply(t, e, event.Entry{Kind: event.KindGameEnded, Round: 1, A: 0, B: 3, Game: 1, Attempt: 2, Winner: &w, Head: "h"})
	mustApply(t, e, event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: 0, B: 3, Score: [2]int{1, 0}})

	entries := e.Log()
	wire, err := event.MarshalLog(entries)
	if err != nil {
		t.Fatalf("MarshalLog: %v", err)
	}
	re, err := event.ReplayLog(wire)
	if err != nil {
		t.Fatalf("ReplayLog: %v", err)
	}
	reWire, err := event.MarshalLog(re.Log())
	if err != nil {
		t.Fatalf("re-MarshalLog: %v", err)
	}
	if !bytes.Equal(wire, reWire) {
		t.Errorf("replayed log differs from the original log:\n%s\nvs\n%s", wire, reWire)
	}
	origState, err := e.StateJSON()
	if err != nil {
		t.Fatalf("StateJSON: %v", err)
	}
	reState, err := re.StateJSON()
	if err != nil {
		t.Fatalf("replayed StateJSON: %v", err)
	}
	if !bytes.Equal(origState, reState) {
		t.Errorf("replayed state differs:\norig: %s\nrepl: %s", origState, reState)
	}
	// The replayed event also reports the accessors identically.
	if got, want := re.NextAttempt(1, 2), e.NextAttempt(1, 2); got != want {
		t.Errorf("replayed NextAttempt = %d, want %d", got, want)
	}
}

// newScriptedEventMidDraft runs the scripted prelude (EventCreated, all 12
// PackOpened) and one pick per seat, leaving the event mid-DRAFT.
func newScriptedEventMidDraft(t *testing.T) *event.Event {
	t.Helper()
	e := event.New()
	mustApply(t, e, event.Entry{Kind: event.KindEventCreated, Seed: 42, Set: "TST", Seats: testSeats()})
	for r := 1; r <= draft.Rounds; r++ {
		for s := 0; s < draft.Seats; s++ {
			mustApply(t, e, event.Entry{Kind: event.KindPackOpened, Round: r, Seat: s, Cards: packCards(r, s)})
		}
	}
	for s := 0; s < draft.Seats; s++ {
		v, ok := e.Next(s)
		if !ok {
			t.Fatalf("seat %d has no pack to pick from mid-draft", s)
		}
		mustApply(t, e, event.Entry{Kind: event.KindPicked, Round: v.Round, PickNo: v.PickNo, Seat: s, ID: v.Pack[0].ID, Auto: true})
	}
	return e
}

// TestLogForRedactsUntilComplete checks the per-seat log redaction.
func TestLogForRedactsUntilComplete(t *testing.T) {
	e := newScriptedEventMidDraft(t)
	if e.Phase() != event.PhaseDraft {
		t.Fatalf("phase is %s, want mid-DRAFT", e.Phase())
	}
	full, err := event.MarshalLog(e.Log())
	if err != nil {
		t.Fatalf("MarshalLog: %v", err)
	}
	seatOne := e.LogFor(1)
	for i := range seatOne {
		en := seatOne[i]
		switch en.Kind {
		case event.KindPackOpened:
			if en.Seat == 1 && len(en.Cards) == 0 {
				t.Errorf("LogFor(1) hid seat 1's own PackOpened cards (entry %d)", i)
			}
			if en.Seat != 1 && len(en.Cards) != 0 {
				t.Errorf("LogFor(1) leaked seat %d's PackOpened cards (entry %d)", en.Seat, i)
			}
		case event.KindPicked:
			if en.Seat == 1 && en.ID == "" {
				t.Errorf("LogFor(1) hid seat 1's own pick ID (entry %d)", i)
			}
			if en.Seat != 1 && en.ID != "" {
				t.Errorf("LogFor(1) leaked seat %d's pick ID %q (entry %d)", en.Seat, en.ID, i)
			}
		}
	}
	// The spectator sees nothing of any seat's picks.
	spec := e.LogFor(-1)
	for i := range spec {
		en := spec[i]
		if en.Kind == event.KindPackOpened && len(en.Cards) != 0 {
			t.Errorf("LogFor(-1) leaked seat %d's pack (entry %d)", en.Seat, i)
		}
		if en.Kind == event.KindPicked && en.ID != "" {
			t.Errorf("LogFor(-1) leaked seat %d's pick ID (entry %d)", en.Seat, i)
		}
	}
	// LogFor never touched the event's own log.
	after, err := event.MarshalLog(e.Log())
	if err != nil {
		t.Fatalf("MarshalLog after LogFor: %v", err)
	}
	if !bytes.Equal(full, after) {
		t.Errorf("e.Log() changed under LogFor")
	}

	// The series phase: GameStarted seeds are hidden from everyone, Table
	// is kept, and the prelude's DeckSaved lists are visible only to their
	// seat.
	r1 := newScriptedEventToRound1(t)
	mustApply(t, r1, event.Entry{Kind: event.KindSeriesStarted, Round: 1, A: 0, B: 3, BestOf: 1})
	attemptStart(t, r1, 1, 0, 3, 1, "t-abc", 99)
	r1v := r1.LogFor(0)
	sawSeed, sawDeck := false, false
	for i := range r1v {
		en := r1v[i]
		if en.Kind == event.KindGameStarted {
			sawSeed = true
			if en.GameSeed != 0 {
				t.Errorf("LogFor(0) leaked GameSeed %d", en.GameSeed)
			}
			if en.Table != "t-abc" {
				t.Errorf("LogFor(0) dropped the attempt table: %q", en.Table)
			}
		}
		if en.Kind == event.KindDeckSaved {
			sawDeck = true
			if en.Seat == 0 && (en.Deck == nil || len(en.Deck.Main) == 0) {
				t.Errorf("LogFor(0) hid seat 0's own deck list (entry %d)", i)
			}
			if en.Seat != 0 && en.Deck != nil {
				t.Errorf("LogFor(0) leaked seat %d's deck list (entry %d)", en.Seat, i)
			}
		}
	}
	if !sawSeed || !sawDeck {
		t.Fatalf("redacted log lacks a GameStarted (%v) or DeckSaved (%v) entry to check", sawSeed, sawDeck)
	}

	// After COMPLETE the spectator log equals the whole log.
	done := newScriptedEvent(t)
	if !done.Done() {
		t.Fatalf("scripted event in phase %s, want COMPLETE", done.Phase())
	}
	want, got := done.Log(), done.LogFor(-1)
	if len(want) != len(got) {
		t.Fatalf("LogFor(-1) after COMPLETE has %d entries, Log() has %d", len(got), len(want))
	}
	for i := range want {
		if want[i].Kind == event.KindPackOpened && len(got[i].Cards) == 0 && len(want[i].Cards) != 0 {
			t.Errorf("LogFor(-1) after COMPLETE still hides seat %d's pack (entry %d)", want[i].Seat, i)
		}
		if want[i].Kind == event.KindPicked && got[i].ID == "" && want[i].ID != "" {
			t.Errorf("LogFor(-1) after COMPLETE still hides seat %d's pick (entry %d)", want[i].Seat, i)
		}
	}
}

// TestAccessorsReadWithoutCreating checks the read-only accessors never
// create series state or move StateJSON.
func TestAccessorsReadWithoutCreating(t *testing.T) {
	e := newScriptedEventToRound1(t)
	before, err := e.StateJSON()
	if err != nil {
		t.Fatalf("StateJSON: %v", err)
	}
	if e.Started(0, 1) {
		t.Error("Started(0,1) = true for a non-pairing")
	}
	if got := e.NextAttempt(0, 1); got != 0 {
		t.Errorf("NextAttempt(0,1) = %d, want 0 for a non-pairing", got)
	}
	if got := e.InFlight(); got != nil {
		t.Errorf("InFlight on a fresh round = %+v, want nil", got)
	}
	// A pairing that has not been started reports attempt 1 without
	// creating state.
	if got := e.NextAttempt(0, 3); got != 1 {
		t.Errorf("NextAttempt(0,3) = %d, want 1", got)
	}
	after, err := e.StateJSON()
	if err != nil {
		t.Fatalf("StateJSON after accessors: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("StateJSON changed under the read-only accessors:\n%s\nvs\n%s", before, after)
	}

	// Results, SavedRev, ReadyRev and Queued.
	if got := e.Results(); len(got) != 0 {
		t.Errorf("Results before any series = %+v, want empty", got)
	}
	for s := 0; s < draft.Seats; s++ {
		if got := e.SavedRev(s); got != 1 {
			t.Errorf("SavedRev(%d) = %d, want 1", s, got)
		}
		if got := e.ReadyRev(s); got != 1 {
			t.Errorf("ReadyRev(%d) = %d, want 1", s, got)
		}
	}
	if got := e.SavedRev(-1); got != 0 {
		t.Errorf("SavedRev(-1) = %d, want 0", got)
	}
	if got := e.ReadyRev(9); got != 0 {
		t.Errorf("ReadyRev(9) = %d, want 0", got)
	}
	full := newScriptedEvent(t)
	res := full.Results()
	if len(res) != len(event.Pairings(1))+len(event.Pairings(2))+len(event.Pairings(3)) {
		t.Fatalf("Results has %d outcomes, want one per played series", len(res))
	}
	if res[0].Round != 1 || res[0].A != 0 || res[0].B != 3 || res[0].Score != [2]int{1, 0} || res[0].Winner != 0 {
		t.Errorf("first result = %+v, want round 1 seats 0–3 won by 0", res[0])
	}
	if got := full.Queued(0); got != 0 {
		t.Errorf("Queued in a COMPLETE event = %d, want 0", got)
	}
}
