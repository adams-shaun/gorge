package event_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/draft"
)

// packSize is the scripted event's pack size: packs of 14 distinct fake
// cards.
const packSize = 14

// packCards builds one fake pack of packSize distinct CardRefs whose IDs
// are unique across the whole event.
func packCards(round, seat int) []limited.CardRef {
	cards := make([]limited.CardRef, packSize)
	for i := range cards {
		id := fmt.Sprintf("r%ds%dc%02d", round, seat, i)
		cards[i] = limited.CardRef{
			ID:     id,
			Name:   "Card " + id,
			Set:    "TST",
			Number: fmt.Sprint(i + 1),
			Rarity: limited.Common,
		}
	}
	return cards
}

// testSeats are four bot entrants.
func testSeats() []event.SeatSpec {
	return []event.SeatSpec{
		{Kind: "bot", Name: "bot-a", Persona: "persona-a"},
		{Kind: "bot", Name: "bot-b", Persona: "persona-b"},
		{Kind: "bot", Name: "bot-c", Persona: "persona-c"},
		{Kind: "bot", Name: "bot-d", Persona: "persona-d"},
	}
}

// mustApply applies en or fails the test.
func mustApply(t *testing.T, e *event.Event, en event.Entry) {
	t.Helper()
	if err := e.Apply(en); err != nil {
		t.Fatalf("Apply(%s %+v): %v", en.Kind, en, err)
	}
}

// scriptedWinner is the scripted result of a series: the winning seat, or
// -1 for the round-3 draw between seats 0 and 1.
func scriptedWinner(round int, a, b int) int {
	switch {
	case round == 1 && a == 0 && b == 3:
		return 0
	case round == 1 && a == 1 && b == 2:
		return 2
	case round == 2 && a == 0 && b == 2:
		return 0
	case round == 2 && a == 1 && b == 3:
		return 3
	case round == 3 && a == 2 && b == 3:
		return 2
	default:
		return -1 // the scripted 0–1 draw in round 3
	}
}

// playScriptedSeries logs one scripted Bo1 series.
func playScriptedSeries(t *testing.T, e *event.Event, round, a, b int) {
	t.Helper()
	mustApply(t, e, event.Entry{Kind: event.KindSeriesStarted, Round: round, A: a, B: b, BestOf: 1})
	w := scriptedWinner(round, a, b)
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

// newScriptedEvent runs the full scripted event end to end: hand-made packs
// of 14 distinct fake CardRefs, a trivial first-card picker, hand-written
// 40-card decks using basics and scripted series results. It fails the test
// on any Apply error or unexpected phase.
func newScriptedEvent(t *testing.T) *event.Event {
	t.Helper()
	e := event.New()
	mustApply(t, e, event.Entry{Kind: event.KindEventCreated, Seed: 42, Set: "TST", Seats: testSeats()})
	for r := 1; r <= draft.Rounds; r++ {
		for s := 0; s < draft.Seats; s++ {
			mustApply(t, e, event.Entry{Kind: event.KindPackOpened, Round: r, Seat: s, Cards: packCards(r, s)})
		}
	}
	for e.Phase() == event.PhaseDraft {
		progressed := false
		for s := 0; s < draft.Seats; s++ {
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
	if e.Phase() != event.PhaseBuild {
		t.Fatalf("draft ended in phase %s, want BUILD", e.Phase())
	}
	for s := 0; s < draft.Seats; s++ {
		pool := e.Pool(s)
		if len(pool) != 3*packSize {
			t.Fatalf("seat %d drafted %d cards, want %d", s, len(pool), 3*packSize)
		}
		d := build.Deck{Main: make([]string, 0, 35), Basics: [5]int{5, 0, 0, 0, 0}}
		for _, c := range pool[:35] {
			d.Main = append(d.Main, c.ID)
		}
		if build.Validate(pool, d) != nil {
			t.Fatalf("seat %d scripted deck invalid: %v", s, build.Validate(pool, d))
		}
		mustApply(t, e, event.Entry{Kind: event.KindDeckSaved, Seat: s, Rev: 1, Deck: &d})
		mustApply(t, e, event.Entry{Kind: event.KindReady, Seat: s, Rev: 1})
	}
	for r := 1; r <= draft.Rounds; r++ {
		for _, p := range event.Pairings(r) {
			playScriptedSeries(t, e, r, p[0], p[1])
		}
	}
	mustApply(t, e, event.Entry{Kind: event.KindEventEnded})
	if !e.Done() {
		t.Fatalf("event ended in phase %s, want COMPLETE", e.Phase())
	}
	return e
}

// TestFullEventLifecycleReachesComplete runs the scripted event, replays
// its log and compares state and log byte for byte.
func TestFullEventLifecycleReachesComplete(t *testing.T) {
	e := newScriptedEvent(t)

	// The scripted results put seat 0 at 7 points (two wins and a draw),
	// seat 2 at 6, seat 3 at 3 and seat 1 at 1.
	st := e.Standings()
	wantOrder := []int{0, 2, 3, 1}
	if len(st) != 4 {
		t.Fatalf("standings has %d rows, want 4", len(st))
	}
	for i, seat := range wantOrder {
		if st[i].Seat != seat {
			t.Fatalf("standings row %d is seat %d, want seat %d", i, st[i].Seat, seat)
		}
	}
	wantPoints := map[int]int{0: 7, 2: 6, 3: 3, 1: 1}
	for i, seat := range wantOrder {
		if st[i].Points != wantPoints[seat] {
			t.Errorf("seat %d has %d points, want %d", seat, st[i].Points, wantPoints[seat])
		}
	}

	// The EventEnded entry in the log carries the computed standings.
	log := e.Log()
	last := log[len(log)-1]
	if last.Kind != event.KindEventEnded || len(last.Standings) != 4 {
		t.Fatalf("last log entry is %+v, want EventEnded with 4 standings rows", last)
	}
	if last.Standings[0].Seat != 0 || last.Standings[0].Points != 7 {
		t.Errorf("logged standings head is %+v, want seat 0 with 7 points", last.Standings[0])
	}

	// Replaying the log yields byte-identical JSON state and log.
	entries := e.Log()
	wire, err := event.MarshalLog(entries)
	if err != nil {
		t.Fatalf("MarshalLog: %v", err)
	}
	re, err := event.Replay(entries)
	if err != nil {
		t.Fatalf("Replay: %v", err)
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

// TestLogJSONRoundTrip marshals entries, parses them back and re-marshals.
func TestLogJSONRoundTrip(t *testing.T) {
	e := newScriptedEvent(t)
	entries := e.Log()
	wire, err := event.MarshalLog(entries)
	if err != nil {
		t.Fatalf("MarshalLog: %v", err)
	}
	parsed, err := event.ParseLog(wire)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	if len(parsed) != len(entries) {
		t.Fatalf("ParseLog returned %d entries, want %d", len(parsed), len(entries))
	}
	rewire, err := event.MarshalLog(parsed)
	if err != nil {
		t.Fatalf("re-MarshalLog: %v", err)
	}
	if !bytes.Equal(wire, rewire) {
		t.Errorf("JSON round trip is not byte-identical")
	}
	parsed2, err := event.ParseLog([]byte("not json\n"))
	if err == nil {
		t.Errorf("ParseLog accepted garbage: %+v", parsed2)
	}
	if _, err := event.ParseLog([]byte(`{"kind":"Nope"}` + "\n")); err == nil {
		t.Errorf("ParseLog accepted an unknown kind")
	}
}

// TestApplyRejectsIllegalEntries checks the phase and value guards.
func TestApplyRejectsIllegalEntries(t *testing.T) {
	e := event.New()

	// Before EventCreated everything else is illegal.
	for _, en := range []event.Entry{
		{Kind: event.KindPackOpened, Round: 1, Seat: 0, Cards: packCards(1, 0)},
		{Kind: event.KindPicked, Round: 1, PickNo: 1, Seat: 0, ID: "x"},
		{Kind: event.KindDeckSaved, Seat: 0, Rev: 1},
		{Kind: event.KindReady, Seat: 0, Rev: 1},
		{Kind: event.KindSeriesStarted, Round: 1, A: 0, B: 3, BestOf: 1},
		{Kind: event.KindEventEnded},
	} {
		if err := e.Apply(en); err == nil {
			t.Errorf("Apply(%s) before EventCreated: want error", en.Kind)
		}
	}
	if err := e.Apply(event.Entry{Kind: event.KindEventCreated, Set: "TST", Seats: testSeats()}); err != nil {
		t.Fatalf("EventCreated: %v", err)
	}
	if e.Phase() != event.PhaseDraft {
		t.Fatalf("phase after EventCreated is %s, want DRAFT", e.Phase())
	}
	if err := e.Apply(event.Entry{Kind: event.KindEventCreated, Set: "TST", Seats: testSeats()}); err == nil {
		t.Errorf("second EventCreated: want error")
	}

	// BUILD-phase entries are illegal in DRAFT.
	if err := e.Apply(event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 1}); err == nil {
		t.Errorf("DeckSaved in DRAFT: want error")
	}
	if err := e.Apply(event.Entry{Kind: event.KindReady, Seat: 0, Rev: 1}); err == nil {
		t.Errorf("Ready in DRAFT: want error")
	}

	// A pick before the packs are opened is rejected.
	if err := e.Apply(event.Entry{Kind: event.KindPicked, Round: 1, PickNo: 1, Seat: 0, ID: "r1s0c00"}); err == nil {
		t.Errorf("Picked before PackOpened: want error")
	}

	// Open round-1 packs only; a pick is still rejected until all 12 are in.
	for s := 0; s < draft.Seats; s++ {
		if err := e.Apply(event.Entry{Kind: event.KindPackOpened, Round: 1, Seat: s, Cards: packCards(1, s)}); err != nil {
			t.Fatalf("PackOpened r1 s%d: %v", s, err)
		}
	}
	if err := e.Apply(event.Entry{Kind: event.KindPicked, Round: 1, PickNo: 1, Seat: 0, ID: "r1s0c00"}); err == nil {
		t.Errorf("Picked with only 4 packs logged: want error")
	}
	// Duplicate and cross-pack duplicate IDs are rejected.
	if err := e.Apply(event.Entry{Kind: event.KindPackOpened, Round: 1, Seat: 0, Cards: packCards(1, 0)}); err == nil {
		t.Errorf("duplicate PackOpened: want error")
	}
	dup := packCards(2, 0)
	dup[0].ID = "r1s0c00"
	if err := e.Apply(event.Entry{Kind: event.KindPackOpened, Round: 2, Seat: 0, Cards: dup}); err == nil {
		t.Errorf("PackOpened reusing another pack's card ID: want error")
	}
	if err := e.Apply(event.Entry{Kind: event.KindPackOpened, Round: 9, Seat: 0, Cards: packCards(9, 0)}); err == nil {
		t.Errorf("PackOpened round 9: want error")
	}

	// Log the remaining packs and drive a full draft on a scratch event so
	// the series-phase guards can be checked with real state.
	full := newScriptedEvent(t)

	// Series entries are illegal outside a round phase.
	if err := full.Apply(event.Entry{Kind: event.KindPicked, Round: 1, PickNo: 1, Seat: 0, ID: "r1s0c00"}); err == nil {
		t.Errorf("Picked in phase %s: want error", full.Phase())
	}
	if err := full.Apply(event.Entry{Kind: event.KindPackOpened, Round: 1, Seat: 1, Cards: packCards(1, 1)}); err == nil {
		t.Errorf("PackOpened in phase %s: want error", full.Phase())
	}
	if err := full.Apply(event.Entry{Kind: event.KindEventEnded}); err == nil {
		t.Errorf("EventEnded in phase %s: want error", full.Phase())
	}

	// On a fresh event driven to ROUND1, the series guards bite.
	r1 := newScriptedEventToRound1(t)
	if e := r1.Phase(); e != event.RoundPhase(1) {
		t.Fatalf("phase is %s, want ROUND1", e)
	}
	if err := r1.Apply(event.Entry{Kind: event.KindSeriesStarted, Round: 1, A: 0, B: 1, BestOf: 1}); err == nil {
		t.Errorf("SeriesStarted for non-pairing 0–1: want error")
	}
	if err := r1.Apply(event.Entry{Kind: event.KindSeriesStarted, Round: 2, A: 0, B: 2, BestOf: 1}); err == nil {
		t.Errorf("SeriesStarted for round 2 in round 1: want error")
	}
	if err := r1.Apply(event.Entry{Kind: event.KindSeriesStarted, Round: 1, A: 3, B: 0, BestOf: 1}); err == nil {
		t.Errorf("SeriesStarted with A above B: want error")
	}
	if err := r1.Apply(event.Entry{Kind: event.KindSeriesStarted, Round: 1, A: 0, B: 3, BestOf: 2}); err == nil {
		t.Errorf("SeriesStarted BestOf 2: want error (only Bo1 and Bo3)")
	}
	if err := r1.Apply(event.Entry{Kind: event.KindGameEnded, Round: 1, A: 0, B: 3, Game: 1, Head: "h"}); err == nil {
		t.Errorf("GameEnded for an unstarted series: want error")
	}
	w := 0
	if err := r1.Apply(event.Entry{Kind: event.KindSeriesStarted, Round: 1, A: 0, B: 3, BestOf: 1}); err != nil {
		t.Fatalf("SeriesStarted 0–3: %v", err)
	}
	if err := r1.Apply(event.Entry{Kind: event.KindSeriesStarted, Round: 1, A: 0, B: 3, BestOf: 1}); err == nil {
		t.Errorf("second SeriesStarted 0–3: want error")
	}
	if err := r1.Apply(event.Entry{Kind: event.KindGameEnded, Round: 1, A: 0, B: 3, Game: 2, Winner: &w, Head: "h"}); err == nil {
		t.Errorf("GameEnded game 2 first: want error")
	}
	bad := 1
	if err := r1.Apply(event.Entry{Kind: event.KindGameEnded, Round: 1, A: 0, B: 3, Game: 1, Winner: &bad, Head: "h"}); err == nil {
		t.Errorf("GameEnded winner seat 1 in series 0–3: want error")
	}
	if err := r1.Apply(event.Entry{Kind: event.KindGameEnded, Round: 1, A: 0, B: 3, Game: 1, Head: ""}); err == nil {
		t.Errorf("GameEnded without a head: want error")
	}
	if err := r1.Apply(event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: 0, B: 3, Score: [2]int{1, 0}}); err == nil {
		t.Errorf("SeriesEnded with no game played: want error")
	}
	if err := r1.Apply(event.Entry{Kind: event.KindGameEnded, Round: 1, A: 0, B: 3, Game: 1, Winner: &w, Head: "h"}); err != nil {
		t.Fatalf("GameEnded 0 beats 3: %v", err)
	}
	if err := r1.Apply(event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: 0, B: 3, Score: [2]int{0, 1}}); err == nil {
		t.Errorf("SeriesEnded with a score not matching the game: want error")
	}
	if err := r1.Apply(event.Entry{Kind: event.KindEventEnded}); err == nil {
		t.Errorf("EventEnded mid-event: want error")
	}
	if err := r1.Apply(event.Entry{Kind: event.KindSeriesEnded, Round: 1, A: 0, B: 3, Score: [2]int{1, 0}}); err != nil {
		t.Fatalf("SeriesEnded 0–3 1–0: %v", err)
	}
	if err := r1.Apply(event.Entry{Kind: event.KindGameEnded, Round: 1, A: 0, B: 3, Game: 2, Winner: &w, Head: "h"}); err == nil {
		t.Errorf("GameEnded after the series ended: want error")
	}
	// The other round-1 series is still pending.
	pending := r1.PendingSeries()
	if len(pending) != 1 || pending[0].A != 1 || pending[0].B != 2 {
		t.Fatalf("PendingSeries after 0–3 ended is %+v, want only 1–2", pending)
	}
}

// newScriptedEventToRound1 runs the scripted prelude up to ROUND1.
func newScriptedEventToRound1(t *testing.T) *event.Event {
	t.Helper()
	e := event.New()
	mustApply(t, e, event.Entry{Kind: event.KindEventCreated, Seed: 7, Set: "TST", Seats: testSeats()})
	for r := 1; r <= draft.Rounds; r++ {
		for s := 0; s < draft.Seats; s++ {
			mustApply(t, e, event.Entry{Kind: event.KindPackOpened, Round: r, Seat: s, Cards: packCards(r, s)})
		}
	}
	for e.Phase() == event.PhaseDraft {
		progressed := false
		for s := 0; s < draft.Seats; s++ {
			v, ok := e.Next(s)
			if !ok {
				continue
			}
			progressed = true
			mustApply(t, e, event.Entry{Kind: event.KindPicked, Round: v.Round, PickNo: v.PickNo, Seat: s, ID: v.Pack[0].ID, Auto: true})
		}
		if !progressed {
			t.Fatalf("draft stalled in phase %s", e.Phase())
		}
	}
	for s := 0; s < draft.Seats; s++ {
		pool := e.Pool(s)
		d := build.Deck{Main: make([]string, 0, 35), Basics: [5]int{5, 0, 0, 0, 0}}
		for _, c := range pool[:35] {
			d.Main = append(d.Main, c.ID)
		}
		mustApply(t, e, event.Entry{Kind: event.KindDeckSaved, Seat: s, Rev: 1, Deck: &d})
		mustApply(t, e, event.Entry{Kind: event.KindReady, Seat: s, Rev: 1})
	}
	return e
}

// TestDeckGuards exercises the BUILD-phase deck rules: a deck that fails
// build.Validate is rejected, a Ready without a saved revision is rejected,
// and a fresh revision clears readiness.
func TestDeckGuards(t *testing.T) {
	e := event.New()
	mustApply(t, e, event.Entry{Kind: event.KindEventCreated, Seed: 7, Set: "TST", Seats: testSeats()})
	for r := 1; r <= draft.Rounds; r++ {
		for s := 0; s < draft.Seats; s++ {
			mustApply(t, e, event.Entry{Kind: event.KindPackOpened, Round: r, Seat: s, Cards: packCards(r, s)})
		}
	}
	for e.Phase() == event.PhaseDraft {
		progressed := false
		for s := 0; s < draft.Seats; s++ {
			v, ok := e.Next(s)
			if !ok {
				continue
			}
			progressed = true
			mustApply(t, e, event.Entry{Kind: event.KindPicked, Round: v.Round, PickNo: v.PickNo, Seat: s, ID: v.Pack[0].ID, Auto: true})
		}
		if !progressed {
			t.Fatalf("draft stalled in phase %s", e.Phase())
		}
	}
	pool := e.Pool(0)
	if len(pool) != 42 {
		t.Fatalf("seat 0 drafted %d cards, want 42", len(pool))
	}

	// A too-small deck fails build.Validate.
	small := build.Deck{Main: []string{pool[0].ID}, Basics: [5]int{5}}
	if err := e.Apply(event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 1, Deck: &small}); err == nil {
		t.Errorf("DeckSaved with a 6-card deck: want error")
	}
	// A maindeck card from another seat's pool is rejected.
	stranger := build.Deck{Main: []string{e.Pool(1)[0].ID}, Basics: [5]int{39}}
	if err := e.Apply(event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 1, Deck: &stranger}); err == nil {
		t.Errorf("DeckSaved with a card from another pool: want error")
	}
	// Revision numbering is enforced.
	ok := build.Deck{Main: []string{pool[0].ID}, Basics: [5]int{39}}
	if err := e.Apply(event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 2, Deck: &ok}); err == nil {
		t.Errorf("DeckSaved rev 2 before rev 1: want error")
	}
	if err := e.Apply(event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 1, Deck: &ok}); err != nil {
		t.Fatalf("DeckSaved rev 1: %v", err)
	}
	if err := e.Apply(event.Entry{Kind: event.KindReady, Seat: 0, Rev: 3}); err == nil {
		t.Errorf("Ready with an unsaved revision: want error")
	}
	if err := e.Apply(event.Entry{Kind: event.KindReady, Seat: 0, Rev: 1}); err != nil {
		t.Fatalf("Ready rev 1: %v", err)
	}
	if err := e.Apply(event.Entry{Kind: event.KindReady, Seat: 0, Rev: 1}); err == nil {
		t.Errorf("second Ready: want error")
	}
	if err := e.Apply(event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 2, Deck: &ok}); err != nil {
		t.Fatalf("DeckSaved rev 2 after readying: %v", err)
	}
	if err := e.Apply(event.Entry{Kind: event.KindEventEnded}); err == nil {
		t.Errorf("EventEnded in phase %s: want error", e.Phase())
	}
}
