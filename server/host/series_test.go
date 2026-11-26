package host

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// seriesTable is the series table every series test plays: two seats, the
// sample decks "a" and "b", public spectators, on demand, owned by event
// "e1", and Series.
func seriesTable(id TableID) TableConfig {
	return TableConfig{ID: id, Name: "Series " + string(id), Seats: 2, Decks: []string{"a", "b"},
		Seed: 99, Spectator: view.Public, OnDemand: true, Event: "e1", Series: true}
}

// planFunc turns a k-indexed table of plans into a SeriesFunc: plan[k-1]
// answers match k; a k past the table ends the series (ok=false).
func planFunc(plans []SeriesGame, failAt int, err error) SeriesFunc {
	return func(ctx context.Context, t TableID, k int) (SeriesGame, bool, error) {
		if failAt == k {
			return SeriesGame{}, false, err
		}
		if k-1 >= len(plans) {
			return SeriesGame{}, false, nil
		}
		return plans[k-1], true, nil
	}
}

// genesisTossNote reads match k's genesis Note (seq 1): its Player is the
// seat posed CR 103.1's choice and its Line names why (the toss, or the
// configured chooser).
func genesisTossNote(t *testing.T, r *Registry, id TableID, k int) (uint8, string) {
	t.Helper()
	evs, err := r.Events(id, k, 0)
	if err != nil {
		t.Fatalf("Events(%s, %d): %v", id, k, err)
	}
	if len(evs) == 0 {
		t.Fatalf("Events(%s, %d): no events", id, k)
	}
	ev := evs[0].Event
	if ev.Seq != 0 || ev.Kind != "game_start" {
		t.Fatalf("match %d's first event is %v seq %d, want game_start at seq 0", k, ev.Kind, ev.Seq)
	}
	// The genesis Note at seq 1 names the seat posed CR 103.1's choice and
	// why: "won the toss" (seeded) or "chooses who takes the first turn"
	// (a configured chooser).
	if evs[1].Event.Kind != "note" || evs[1].Event.Seq != 1 {
		t.Fatalf("match %d's second event is %v seq %d, want the genesis Note at seq 1", k, evs[1].Event.Kind, evs[1].Event.Seq)
	}
	note := evs[1]
	return note.Event.Player, note.Line
}

func TestSeriesTablePlaysEachPlannedMatch(t *testing.T) {
	t.Parallel()
	takeMatchSlot(t)
	o := testOptions(t)
	o.Series = planFunc([]SeriesGame{
		{Decks: []string{"a", "b"}, Chooser: -1},
		{Decks: []string{"c", "b"}, Chooser: 1},
		{Decks: []string{"a", "d"}, Chooser: 0},
	}, 0, nil)
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(seriesTable("t1")); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")

	ms, err := r.Matches("t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 3 {
		t.Fatalf("Matches has %d entries, want the 3 planned matches", len(ms))
	}
	// The precondition every chooser read below leans on: match k really
	// played the plan's decks, seat by seat, and finished.
	wantDecks := [][]string{{"a", "b"}, {"c", "b"}, {"a", "d"}}
	for i, m := range ms {
		if m.Match != i+1 {
			t.Fatalf("entry %d is match %d, want %d", i, m.Match, i+1)
		}
		if m.State != protocol.MatchFinished {
			t.Fatalf("match %d is %s, want finished", m.Match, m.State)
		}
		for seat, want := range wantDecks[i] {
			if got := m.Seats[seat].DeckID; got != want {
				t.Fatalf("match %d seat %d plays deck %q, want %q", m.Match, seat, got, want)
			}
		}
	}
	for _, ti := range r.Tables() {
		if ti.ID == "t1" && ti.State != protocol.TableIdle {
			t.Fatalf("table state %s, want idle after the hook said stop", ti.State)
		}
	}
	// Match 1 kept the seeded toss: the genesis Note names the toss winner.
	if player, line := genesisTossNote(t, r, "t1", 1); player != uint8(MatchTossWinner(99, 1, 2)) || !strings.HasSuffix(line, "won the toss") {
		t.Fatalf("match 1 genesis note: player %d line %q, want toss winner %d ending \"won the toss\"", player, line, MatchTossWinner(99, 1, 2))
	}
	// Matches 2 and 3 pose the choice to the configured chooser instead.
	for k, want := range map[int]uint8{2: 1, 3: 0} {
		if player, line := genesisTossNote(t, r, "t1", k); player != want || !strings.HasSuffix(line, "chooses who takes the first turn") {
			t.Fatalf("match %d genesis note: player %d line %q, want chooser %d ending \"chooses who takes the first turn\"", k, player, line, want)
		}
	}
}

func TestSeriesHookCalledWithNoLocksAndCloseUnblocks(t *testing.T) {
	t.Parallel()
	takeMatchSlot(t)
	o := testOptions(t)
	k2 := make(chan struct{}, 1)
	o.Series = func(ctx context.Context, t TableID, k int) (SeriesGame, bool, error) {
		if k == 1 {
			return SeriesGame{Decks: []string{"a", "b"}, Chooser: -1}, true, nil
		}
		k2 <- struct{}{}
		// Block the way a sideboarding wait does, until Close cancels ctx.
		<-ctx.Done()
		return SeriesGame{}, false, nil
	}
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(seriesTable("t1")); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	<-k2
	// While the hook is blocked it must hold no Registry lock (Tables
	// answers) and no table lock, and the table must already show the
	// between-games state.
	answered := make(chan protocol.TableInfo, 1)
	go func() {
		for _, ti := range r.Tables() {
			if ti.ID == "t1" {
				answered <- ti
				return
			}
		}
	}()
	var info protocol.TableInfo
	select {
	case info = <-answered:
	case <-time.After(5 * time.Second):
		t.Fatal("Tables() did not answer while the hook was blocked (a lock is held)")
	}
	if info.State != protocol.TableCooldown {
		t.Fatalf("table state %s while the hook blocks, want cooldown", info.State)
	}
	closed := make(chan error, 1)
	go func() { closed <- r.Close() }()
	select {
	case <-closed:
	case <-time.After(testutil.WaitBudget):
		t.Fatal("Close did not return while the hook blocked on ctx")
	}
}

func TestSeriesHookErrorHaltsAndStopEndsIdle(t *testing.T) {
	t.Parallel()
	takeMatchSlot(t)
	o := testOptions(t)
	o.Series = func(ctx context.Context, t TableID, k int) (SeriesGame, bool, error) {
		switch t {
		case "t1":
			if k == 1 {
				return SeriesGame{Decks: []string{"a", "b"}, Chooser: -1}, true, nil
			}
			return SeriesGame{}, false, errors.New("boom")
		default: // t2
			return SeriesGame{}, false, nil
		}
	}
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(seriesTable("t1")); err != nil {
		t.Fatal(err)
	}
	if err := r.AddTable(seriesTable("t2")); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t2"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	r.Wait("t2")

	r.mu.RLock()
	t1 := r.tables["t1"]
	t2 := r.tables["t2"]
	r.mu.RUnlock()
	t1.mu.RLock()
	state1, reason1 := t1.state, t1.reason
	t1.mu.RUnlock()
	if state1 != protocol.TableHalted {
		t.Fatalf("t1 state %s, want halted after the hook errored", state1)
	}
	if !strings.Contains(reason1, "boom") {
		t.Fatalf("halt reason %q does not name the hook's error", reason1)
	}
	ms, err := r.Matches("t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 {
		t.Fatalf("t1 has %d matches, want only the one played before the error", len(ms))
	}
	t2.mu.RLock()
	state2 := t2.state
	t2.mu.RUnlock()
	if state2 != protocol.TableIdle {
		t.Fatalf("t2 state %s, want idle after the hook said stop", state2)
	}
	ms2, err := r.Matches("t2")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms2) != 0 {
		t.Fatalf("t2 played %d matches, want zero", len(ms2))
	}
}

func TestSeriesTableValidation(t *testing.T) {
	t.Parallel()
	takeMatchSlot(t)
	// A registry without Options.Series refuses a Series table at
	// registration.
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(seriesTable("t1")); err == nil || !strings.Contains(err.Error(), "a series table needs Options.Series") {
		t.Fatalf("AddTable without Options.Series: %v, want the needs-Options.Series refusal", err)
	}
	// A Series table with no Event is refused at validation.
	o := testOptions(t)
	o.Series = planFunc(nil, 0, nil)
	r2, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Close()
	noEvent := seriesTable("t3")
	noEvent.Event = ""
	if err := r2.AddTable(noEvent); err == nil || !strings.Contains(err.Error(), "needs an event") {
		t.Fatalf("AddTable Series without Event: %v, want the needs-an-event refusal", err)
	}
	// A bad plan (one deck for two seats; a chooser outside the seats)
	// halts the table at that match.
	bad := []SeriesGame{
		{Decks: []string{"a"}, Chooser: -1},
		{Decks: []string{"a", "b"}, Chooser: 2},
	}
	for i, g := range bad {
		o2 := testOptions(t)
		o2.Series = func(ctx context.Context, t TableID, k int) (SeriesGame, bool, error) {
			return g, true, nil
		}
		r3, err := New(o2)
		if err != nil {
			t.Fatal(err)
		}
		defer r3.Close()
		id := TableID(string('a' + rune(i)))
		if err := r3.AddTable(seriesTable(id)); err != nil {
			t.Fatal(err)
		}
		if err := r3.Start(id); err != nil {
			t.Fatal(err)
		}
		r3.Wait(id)
		for _, ti := range r3.Tables() {
			if ti.ID == string(id) && ti.State != protocol.TableHalted {
				t.Fatalf("plan %+v: table state %s, want halted", g, ti.State)
			}
		}
		ms, err := r3.Matches(id)
		if err != nil {
			t.Fatal(err)
		}
		if len(ms) != 0 {
			t.Fatalf("plan %+v played %d matches, want zero", g, len(ms))
		}
	}
}

func TestSeriesMatchServesFromDisk(t *testing.T) {
	t.Parallel()
	takeMatchSlot(t)
	o := testOptions(t)
	o.Dir = t.TempDir()
	o.Series = planFunc([]SeriesGame{
		{Decks: []string{"a", "b"}, Chooser: -1},
		{Decks: []string{"c", "b"}, Chooser: 1},
	}, 0, nil)
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(seriesTable("t1")); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	ms, err := r.Matches("t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 {
		t.Fatalf("Matches has %d entries, want the 2 planned matches", len(ms))
	}
	if ms[1].Match != 2 {
		t.Fatalf("entry 1 is match %d, want 2", ms[1].Match)
	}
	// The recorded chooser reaches the disk replay: matchForLog rebuilds the
	// rules.Config from the sidecar alone, and a replay without the chooser
	// would replay the toss instead and refuse the log at the genesis Note.
	evs, err := r.Events("t1", 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) == 0 {
		t.Fatal("match 2 has no events")
	}
	if v, err := r.ViewAt("t1", 2, evs[len(evs)-1].Event.Seq); err != nil {
		t.Fatalf("ViewAt(match 2, last seq): %v (the sidecar's start_chooser did not rebuild a replayable Config)", err)
	} else if v.Visibility == "" {
		t.Fatal("ViewAt returned an empty view")
	}
	// The sidecar carries the chooser exactly when one was configured. The
	// file is MarshalIndent, so the check decodes the JSON rather than
	// matching a spelling.
	raw2, err := os.ReadFile(filepath.Join(o.Dir, "t1", "2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var sc2 map[string]any
	if err := json.Unmarshal(raw2, &sc2); err != nil {
		t.Fatal(err)
	}
	if n, ok := sc2["start_chooser"].(float64); !ok || int(n) != 1 {
		t.Fatalf("2.json does not carry start_chooser 1: got %v", sc2["start_chooser"])
	}
	raw1, err := os.ReadFile(filepath.Join(o.Dir, "t1", "1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var sc1 map[string]any
	if err := json.Unmarshal(raw1, &sc1); err != nil {
		t.Fatal(err)
	}
	if _, present := sc1["start_chooser"]; present {
		t.Fatalf("1.json (toss match) carries start_chooser: %v", sc1["start_chooser"])
	}
}

func TestFeedbackCarriesStartChooser(t *testing.T) {
	chooser := 1
	sc := sidecar{Table: "t1", Match: 2, Decks: []string{"a", "b"}, State: "finished", StartChooser: &chooser}
	fm := feedbackMatch(sc, [][]string{{"x"}, {"y"}}, nil, nil, nil)
	if fm.StartChooser == nil || *fm.StartChooser != 1 {
		t.Fatalf("feedbackMatch dropped StartChooser: %v", fm.StartChooser)
	}
	// The precondition the key test below leans on: a nil chooser marshals
	// no key at all, so old snapshots stay byte-comparable.
	scNil := sc
	scNil.StartChooser = nil
	fmNil := feedbackMatch(scNil, [][]string{{"x"}, {"y"}}, nil, nil, nil)
	if fmNil.StartChooser != nil {
		t.Fatal("feedbackMatch invented a chooser for a nil sidecar field")
	}
	raw, err := json.Marshal(fmNil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "start_chooser") {
		t.Fatalf("a nil chooser marshalled a start_chooser key: %s", raw)
	}
}
