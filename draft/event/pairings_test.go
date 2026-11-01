package event_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/build"
)

// eightSeats are eight bot entrants.
func eightSeats() []event.SeatSpec {
	out := make([]event.SeatSpec, 8)
	for i := range out {
		out[i] = event.SeatSpec{Kind: "bot", Name: fmt.Sprintf("bot-%d", i), Persona: fmt.Sprintf("persona-%d", i)}
	}
	return out
}

// deckFor builds a valid 40-card deck (35 pool maindeck cards + 5 basics)
// from seat's pool.
func deckFor(t *testing.T, e *event.Event, seat int) build.Deck {
	t.Helper()
	pool := e.Pool(seat)
	if len(pool) < 35 {
		t.Fatalf("seat %d pool has %d cards, want at least 35", seat, len(pool))
	}
	d := build.Deck{Main: make([]string, 0, 35), Basics: [5]int{5, 0, 0, 0, 0}}
	for _, c := range pool[:35] {
		d.Main = append(d.Main, c.ID)
	}
	if err := build.Validate(pool, d); err != nil {
		t.Fatalf("seat %d scripted deck invalid: %v", seat, err)
	}
	return d
}

// saveAndReadyAll saves deck revision 1 for every seat and readies it.
func saveAndReadyAll(t *testing.T, e *event.Event) {
	t.Helper()
	for s := 0; s < e.NumSeats(); s++ {
		d := deckFor(t, e, s)
		mustApply(t, e, event.Entry{Kind: event.KindDeckSaved, Seat: s, Rev: 1, Deck: &d})
		mustApply(t, e, event.Entry{Kind: event.KindReady, Seat: s, Rev: 1})
	}
}

// driveDraftPicks drives the supply phase's draft with a first-card picker
// until the phase leaves DRAFT.
func driveDraftPicks(t *testing.T, e *event.Event) {
	t.Helper()
	for e.Phase() == event.PhaseDraft {
		progressed := false
		for s := 0; s < e.NumSeats(); s++ {
			v, ok := e.Next(s)
			if !ok {
				continue
			}
			progressed = true
			if len(v.Pack) == 0 {
				t.Fatalf("seat %d offered an empty pack", s)
			}
			mustApply(t, e, event.Entry{Kind: event.KindPicked, Round: v.Round, PickNo: v.PickNo, Seat: s, ID: v.Pack[0].ID, Auto: true})
		}
		if !progressed {
			t.Fatalf("draft stalled in phase %s with no pickable seat", e.Phase())
		}
	}
}

// playBo1 logs one scripted Bo1 series whose winner is w (-1 for a draw).
func playBo1(t *testing.T, e *event.Event, round, a, b, w int) {
	t.Helper()
	mustApply(t, e, event.Entry{Kind: event.KindSeriesStarted, Round: round, A: a, B: b, BestOf: 1})
	ge := event.Entry{Kind: event.KindGameEnded, Round: round, A: a, B: b, Game: 1, Head: fmt.Sprintf("head-r%ds%d-%d", round, a, b)}
	if w >= 0 {
		ge.Winner = &w
	}
	mustApply(t, e, ge)
	score := [2]int{}
	switch {
	case w == a:
		score[0] = 1
	case w == b:
		score[1] = 1
	}
	mustApply(t, e, event.Entry{Kind: event.KindSeriesEnded, Round: round, A: a, B: b, Score: score})
}

// playAllRoundRobin plays every series of every event round; win names the
// winning seat of each (round, a, b) series, -1 for a draw.
func playAllRoundRobin(t *testing.T, e *event.Event, win func(round, a, b int) int) {
	t.Helper()
	for r := 1; r <= e.Rounds(); r++ {
		if e.Phase() != event.RoundPhase(r) {
			t.Fatalf("round %d expected, phase is %s", r, e.Phase())
		}
		for _, p := range e.CurrentPairings() {
			playBo1(t, e, r, p[0], p[1], win(r, p[0], p[1]))
		}
	}
}

// lowerSeatWins is the 8-seat script: the lower seat of each pairing wins.
func lowerSeatWins(round, a, b int) int { return a }

// newEightSeatDraft runs an 8-seat, 3-pack draft to BUILD with synthetic
// packs and a first-card picker.
func newEightSeatDraft(t *testing.T, en event.Entry) *event.Event {
	t.Helper()
	e := event.New()
	en.Kind = event.KindEventCreated
	en.Set = "TST"
	if en.Seed == 0 {
		en.Seed = 42
	}
	if len(en.Seats) == 0 {
		en.Seats = eightSeats()
	}
	mustApply(t, e, en)
	for r := 1; r <= event.DraftPacks; r++ {
		for s := 0; s < e.NumSeats(); s++ {
			mustApply(t, e, event.Entry{Kind: event.KindPackOpened, Round: r, Seat: s, Cards: packCards(r, s)})
		}
	}
	driveDraftPicks(t, e)
	if e.Phase() != event.PhaseBuild {
		t.Fatalf("draft ended in phase %s, want BUILD", e.Phase())
	}
	return e
}

// replayMatches asserts the event's log replays to an event with the same
// StateJSON and the same log bytes.
func replayMatches(t *testing.T, e *event.Event) {
	t.Helper()
	entries := e.Log()
	wire, err := event.MarshalLog(entries)
	if err != nil {
		t.Fatalf("MarshalLog: %v", err)
	}
	re, err := event.ReplayLog(wire)
	if err != nil {
		t.Fatalf("ReplayLog: %v", err)
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
	reLog, err := event.MarshalLog(re.Log())
	if err != nil {
		t.Fatalf("MarshalLog(replay): %v", err)
	}
	if !bytes.Equal(wire, reLog) {
		t.Errorf("replayed log differs from the original log")
	}
}

// TestEightSeatDraftRoundRobin runs a full 8-seat draft round robin: 7
// rounds of 4 series, lower seat winning, ending COMPLETE with seat 0 at 21
// points, and a log that replays byte-identically.
func TestEightSeatDraftRoundRobin(t *testing.T) {
	e := newEightSeatDraft(t, event.Entry{})
	if e.NumSeats() != 8 || e.Mode() != event.ModeDraft || e.Structure() != event.StructureRoundRobin {
		t.Fatalf("shape is %d seats, mode %q, structure %q", e.NumSeats(), e.Mode(), e.Structure())
	}
	if e.Rounds() != 7 {
		t.Fatalf("rounds is %d, want 7 (n-1 default)", e.Rounds())
	}
	for s := 0; s < 8; s++ {
		if got := len(e.Pool(s)); got != 3*packSize {
			t.Fatalf("seat %d drafted %d cards, want %d", s, got, 3*packSize)
		}
	}
	saveAndReadyAll(t, e)
	if e.Phase() != event.RoundPhase(1) {
		t.Fatalf("phase after readying is %s, want ROUND1", e.Phase())
	}
	playAllRoundRobin(t, e, lowerSeatWins)
	mustApply(t, e, event.Entry{Kind: event.KindEventEnded})
	if !e.Done() {
		t.Fatalf("event ended in phase %s, want COMPLETE", e.Phase())
	}
	st := e.Standings()
	if len(st) != 8 {
		t.Fatalf("standings has %d rows, want 8", len(st))
	}
	for i, row := range st {
		if row.Seat != i {
			t.Fatalf("standings row %d is seat %d, want seat %d", i, row.Seat, i)
		}
		if want := 3 * (7 - i); row.Points != want {
			t.Errorf("seat %d has %d points, want %d", row.Seat, row.Points, want)
		}
	}
	if st[0].Seat != 0 || st[0].Points != 21 {
		t.Fatalf("standings head is %+v, want seat 0 with 21 points", st[0])
	}
	replayMatches(t, e)
}

// TestEightSeatRoundRobinCustomRounds proves an explicit Rounds shortens
// the series: EventEnded is refused in round 2's aftermath and legal once
// round 3 is played.
func TestEightSeatRoundRobinCustomRounds(t *testing.T) {
	e := newEightSeatDraft(t, event.Entry{Rounds: 3})
	saveAndReadyAll(t, e)
	if e.Rounds() != 3 {
		t.Fatalf("rounds is %d, want 3", e.Rounds())
	}
	for r := 1; r <= 2; r++ {
		for _, p := range e.CurrentPairings() {
			playBo1(t, e, r, p[0], p[1], p[0])
		}
	}
	if e.Phase() != event.RoundPhase(3) {
		t.Fatalf("phase after round 2 is %s, want ROUND3", e.Phase())
	}
	if err := e.Apply(event.Entry{Kind: event.KindEventEnded}); err == nil {
		t.Errorf("EventEnded before round 3: want error")
	}
	for _, p := range e.CurrentPairings() {
		playBo1(t, e, 3, p[0], p[1], p[0])
	}
	if err := e.Apply(event.Entry{Kind: event.KindEventEnded}); err != nil {
		t.Fatalf("EventEnded after round 3: %v", err)
	}
	if !e.Done() {
		t.Fatalf("event ended in phase %s, want COMPLETE", e.Phase())
	}
	replayMatches(t, e)
}

// TestRoundRobinPairingsN pins the exported circle-method pairing
// generator: 4 seats matches Pairings, 8 seats meet exactly once over 7
// perfect-matching rounds, and bad shapes return nil.
func TestRoundRobinPairingsN(t *testing.T) {
	for r := 1; r <= 3; r++ {
		got := event.RoundRobinPairings(4, r)
		want := event.Pairings(r)
		if len(got) != len(want) {
			t.Fatalf("n=4 round %d has %d pairs, Pairings has %d", r, len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("n=4 round %d pair %d is %v, want %v", r, i, got[i], want[i])
			}
		}
	}
	for _, bad := range [][2]int{{5, 1}, {8, 0}, {8, 8}, {3, 1}, {0, 1}} {
		if got := event.RoundRobinPairings(bad[0], bad[1]); got != nil {
			t.Errorf("RoundRobinPairings(%d, %d) = %v, want nil", bad[0], bad[1], got)
		}
	}
	seen := map[[2]int]int{}
	for r := 1; r <= 7; r++ {
		got := event.RoundRobinPairings(8, r)
		if len(got) != 4 {
			t.Fatalf("n=8 round %d has %d pairs, want 4", r, len(got))
		}
		covered := map[int]bool{}
		for _, p := range got {
			if p[0] >= p[1] {
				t.Errorf("round %d pair %v is not lower seat first", r, p)
			}
			if p[0] < 0 || p[1] > 7 {
				t.Errorf("round %d pair %v out of range", r, p)
			}
			if covered[p[0]] || covered[p[1]] {
				t.Errorf("round %d is not a perfect matching: %v", r, got)
			}
			covered[p[0]], covered[p[1]] = true, true
			seen[p]++
		}
		if len(covered) != 8 {
			t.Fatalf("round %d covers %d seats, want 8", r, len(covered))
		}
	}
	if len(seen) != 28 {
		t.Fatalf("the 8-seat table holds %d distinct pairs, want 28", len(seen))
	}
	for p, n := range seen {
		if n != 1 {
			t.Errorf("pair %v meets %d times, want exactly once", p, n)
		}
	}
}
