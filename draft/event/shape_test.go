package event_test

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
)

// TestEventCreatedShapeValidation checks every EventCreated shape refusal
// and that a refusal leaves the log empty, plus the normalized accessors.
func TestEventCreatedShapeValidation(t *testing.T) {
	bad := []struct {
		name string
		en   event.Entry
		want string
	}{
		{"5 seats", event.Entry{Set: "TST", Seats: append(append([]event.SeatSpec{}, testSeats()...), event.SeatSpec{Kind: "bot", Name: "x"})}, "got 5 seats, want 4 or 8"},
		{"2 seats", event.Entry{Set: "TST", Seats: testSeats()[:2]}, "got 2 seats, want 4 or 8"},
		{"unknown mode", event.Entry{Set: "TST", Seats: testSeats(), Mode: "cube"}, `unknown mode "cube"`},
		{"draft packs 6", event.Entry{Set: "TST", Seats: testSeats(), Packs: 6}, "draft packs 6, want 3"},
		{"sealed packs 13", event.Entry{Set: "TST", Seats: testSeats(), Mode: "sealed", Packs: 13}, "sealed packs 13, want 1..12"},
		{"unknown structure", event.Entry{Set: "TST", Seats: testSeats(), Structure: "ladder"}, `unknown structure "ladder"`},
		{"rounds 8 at 8 seats", event.Entry{Set: "TST", Seats: eightSeats(), Rounds: 8}, "rounds 8 out of range 1..7"},
		{"none with rounds", event.Entry{Set: "TST", Seats: testSeats(), Structure: "none", Rounds: 1}, "structure none takes no rounds"},
	}
	for _, tc := range bad {
		e := event.New()
		en := tc.en
		en.Kind = event.KindEventCreated
		if err := e.Apply(en); err == nil {
			t.Errorf("%s: Apply: want error", tc.name)
		} else if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q, want it to contain %q", tc.name, err, tc.want)
		}
		if log := e.Log(); len(log) != 0 {
			t.Errorf("%s: refusal logged %d entries, want none", tc.name, len(log))
		}
		if e.Phase() != event.PhaseLobby {
			t.Errorf("%s: phase is %s after a refusal, want LOBBY", tc.name, e.Phase())
		}
		if e.NumSeats() != 0 {
			t.Errorf("%s: NumSeats is %d after a refusal, want 0", tc.name, e.NumSeats())
		}
	}

	// The accepted shapes normalize to their accessors.
	ok := []struct {
		name   string
		en     event.Entry
		seats  int
		mode   string
		struc  string
		rounds int
		packs  int
	}{
		{"default draft", event.Entry{Seats: testSeats()}, 4, event.ModeDraft, event.StructureRoundRobin, 3, event.DraftPacks},
		{"draft packs 3", event.Entry{Seats: testSeats(), Packs: 3}, 4, event.ModeDraft, event.StructureRoundRobin, 3, 3},
		{"sealed default", event.Entry{Seats: testSeats(), Mode: "sealed"}, 4, event.ModeSealed, event.StructureRoundRobin, 3, event.DefaultSealedPacks},
		{"sealed packs 12", event.Entry{Seats: testSeats(), Mode: "sealed", Packs: 12}, 4, event.ModeSealed, event.StructureRoundRobin, 3, 12},
		{"8 seats roundrobin", event.Entry{Seats: eightSeats()}, 8, event.ModeDraft, event.StructureRoundRobin, 7, event.DraftPacks},
		{"8 seats custom rounds", event.Entry{Seats: eightSeats(), Rounds: 3}, 8, event.ModeDraft, event.StructureRoundRobin, 3, event.DraftPacks},
		{"structure none", event.Entry{Seats: eightSeats(), Structure: "none"}, 8, event.ModeDraft, event.StructureNone, 0, event.DraftPacks},
		{"4 seats swiss", event.Entry{Seats: testSeats(), Structure: "swiss"}, 4, event.ModeDraft, event.StructureSwiss, 2, event.DraftPacks},
		{"8 seats swiss", event.Entry{Seats: eightSeats(), Structure: "swiss"}, 8, event.ModeDraft, event.StructureSwiss, 3, event.DraftPacks},
		{"collation logged", event.Entry{Seats: testSeats(), Collation: "fdn-premium"}, 4, event.ModeDraft, event.StructureRoundRobin, 3, event.DraftPacks},
	}
	for _, tc := range ok {
		e := event.New()
		en := tc.en
		en.Kind = event.KindEventCreated
		en.Set = "TST"
		if err := e.Apply(en); err != nil {
			t.Fatalf("%s: Apply: %v", tc.name, err)
		}
		if e.NumSeats() != tc.seats || e.Mode() != tc.mode || e.Structure() != tc.struc ||
			e.Rounds() != tc.rounds || e.PacksPerSeat() != tc.packs {
			t.Errorf("%s: shape is %d seats, mode %q, structure %q, rounds %d, packs %d",
				tc.name, e.NumSeats(), e.Mode(), e.Structure(), e.Rounds(), e.PacksPerSeat())
		}
	}
}

// TestEventCreatedOmitsDefaultShape pins that an EventCreated with no shape
// fields marshals without any of them.
func TestEventCreatedOmitsDefaultShape(t *testing.T) {
	e := event.New()
	if err := e.Apply(event.Entry{Kind: event.KindEventCreated, Seed: 42, Set: "TST", Seats: testSeats()}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	wire, err := event.MarshalLog(e.Log())
	if err != nil {
		t.Fatalf("MarshalLog: %v", err)
	}
	for _, frag := range []string{`"mode"`, `"structure"`, `"packs"`, `"rounds"`, `"collation"`} {
		if strings.Contains(string(wire), frag) {
			t.Errorf("old-shape EventCreated log contains %s:\n%s", frag, wire)
		}
	}
	parsed, err := event.ParseLog(wire)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	if parsed[0].Mode != "" || parsed[0].Structure != "" || parsed[0].Packs != 0 || parsed[0].Rounds != 0 || parsed[0].Collation != "" {
		t.Errorf("parsed shape fields not empty: %+v", parsed[0])
	}
}

// TestStructureNoneEndsAfterBuild proves a no-games event ends from BUILD
// once every seat is ready, with a zero standings table.
func TestStructureNoneEndsAfterBuild(t *testing.T) {
	e := newEightSeatDraft(t, event.Entry{Structure: "none"})
	if e.Structure() != event.StructureNone || e.Rounds() != 0 || e.Round() != 0 {
		t.Fatalf("shape is structure %q, rounds %d, round %d", e.Structure(), e.Rounds(), e.Round())
	}
	if e.Phase() != event.PhaseBuild {
		t.Fatalf("phase is %s, want BUILD", e.Phase())
	}
	if ps := e.PendingSeries(); len(ps) != 0 {
		t.Fatalf("PendingSeries is %+v, want empty", ps)
	}
	// EventEnded is refused until every seat has readied.
	for s := 0; s < 7; s++ {
		d := deckFor(t, e, s)
		mustApply(t, e, event.Entry{Kind: event.KindDeckSaved, Seat: s, Rev: 1, Deck: &d})
		mustApply(t, e, event.Entry{Kind: event.KindReady, Seat: s, Rev: 1})
	}
	if e.Phase() != event.PhaseBuild {
		t.Fatalf("phase after 7 readies is %s, want BUILD", e.Phase())
	}
	if err := e.Apply(event.Entry{Kind: event.KindEventEnded}); err == nil {
		t.Errorf("EventEnded with an unreadied seat: want error")
	}
	d := deckFor(t, e, 7)
	mustApply(t, e, event.Entry{Kind: event.KindDeckSaved, Seat: 7, Rev: 1, Deck: &d})
	mustApply(t, e, event.Entry{Kind: event.KindReady, Seat: 7, Rev: 1})
	if e.Phase() != event.PhaseBuild {
		t.Fatalf("phase after all 8 readies is %s, want BUILD", e.Phase())
	}
	if ps := e.PendingSeries(); len(ps) != 0 {
		t.Fatalf("PendingSeries after readying is %+v, want empty", ps)
	}
	if err := e.Apply(event.Entry{Kind: event.KindEventEnded}); err != nil {
		t.Fatalf("EventEnded from BUILD: %v", err)
	}
	if !e.Done() {
		t.Fatalf("event ended in phase %s, want COMPLETE", e.Phase())
	}
	st := e.Standings()
	if len(st) != 8 {
		t.Fatalf("standings has %d rows, want 8", len(st))
	}
	for i, row := range st {
		if row.Seat != i || row.Points != 0 || row.MW != 0 || row.OMW != 0 || row.GW != 0 || row.OGW != 0 {
			t.Errorf("standings row %d is %+v, want an all-zero row for seat %d", i, row, i)
		}
	}
	replayMatches(t, e)
}

// TestComputeStandingsN checks the n-row standings builder: 8 seats rank
// the 4..7 results, and ComputeStandings still ignores seats >= 4.
func TestComputeStandingsN(t *testing.T) {
	w45, w67 := 4, 6
	results := []event.SeriesResult{
		{Round: 1, A: 4, B: 5, Score: [2]int{1, 0}, Games: []event.GameOutcome{event.AWon}},
		{Round: 1, A: 6, B: 7, Score: [2]int{1, 0}, Games: []event.GameOutcome{event.AWon}},
	}
	st := event.ComputeStandingsN(8, results)
	if len(st) != 8 {
		t.Fatalf("standings has %d rows, want 8", len(st))
	}
	seats := map[int]bool{}
	for _, row := range st {
		seats[row.Seat] = true
	}
	for s := 0; s <= 7; s++ {
		if !seats[s] {
			t.Errorf("standings is missing seat %d", s)
		}
	}
	if st[0].Seat != w45 || st[0].Points != 3 {
		t.Errorf("standings head is %+v, want seat %d with 3 points", st[0], w45)
	}
	if st[1].Seat != w67 || st[1].Points != 3 {
		t.Errorf("standings row 1 is %+v, want seat %d with 3 points", st[1], w67)
	}

	// The 4-seat form still ignores any seat >= 4.
	if got := event.ComputeStandings(results); len(got) != 4 {
		t.Fatalf("ComputeStandings has %d rows, want 4", len(got))
	}
	ignored := []event.SeriesResult{{Round: 1, A: 0, B: 5, Score: [2]int{1, 0}}}
	st4 := event.ComputeStandings(ignored)
	if len(st4) != 4 {
		t.Fatalf("ComputeStandings has %d rows, want 4", len(st4))
	}
	for _, row := range st4 {
		if row.Points != 0 {
			t.Errorf("row %+v has points, want a seat 0–5 result to be ignored", row)
		}
	}
}

// TestShapeAccessorsBeforeCreation checks the zero shape: nothing is ready
// and no seats exist before EventCreated.
func TestShapeAccessorsBeforeCreation(t *testing.T) {
	e := event.New()
	if e.NumSeats() != 0 || e.Mode() != "" || e.Structure() != "" || e.Rounds() != 0 || e.PacksPerSeat() != 0 || e.Collation() != "" {
		t.Errorf("fresh event shape is %d seats, mode %q, structure %q, rounds %d, packs %d, collation %q",
			e.NumSeats(), e.Mode(), e.Structure(), e.Rounds(), e.PacksPerSeat(), e.Collation())
	}
	if _, ok := e.Next(0); ok {
		t.Errorf("Next(0) offered a pack before creation")
	}
	if pool := e.Pool(0); pool != nil {
		t.Errorf("Pool(0) is %v before creation, want nil", pool)
	}
	if cp := e.CurrentPairings(); cp != nil {
		t.Errorf("CurrentPairings is %v before creation, want nil", cp)
	}
}
