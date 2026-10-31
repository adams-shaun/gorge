package host

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/protocol"
)

// TestTableInfoSeriesFlag is E3-fx2's first half: TableInfo carries the
// table's series flag so a client can tell a series table (which autoplays
// until its hook says stop) from an ordinary one. The wire omits the key for
// a non-series table (omitempty), so an old client sees no change.
func TestTableInfoSeriesFlag(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	o.Series = planFunc(nil, 0, nil)
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(seriesTable("t1")); err != nil {
		t.Fatal(err)
	}
	if err := r.AddTable(fourSeatTable("t2", false)); err != nil {
		t.Fatal(err)
	}
	var series, plain protocol.TableInfo
	for _, ti := range r.Tables() {
		switch ti.ID {
		case "t1":
			series = ti
		case "t2":
			plain = ti
		}
	}
	// Precondition: both tables were actually built and found, so a missing
	// Series field cannot pass by both rows being the zero TableInfo.
	if series.ID != "t1" || plain.ID != "t2" {
		t.Fatalf("Tables() returned series=%q plain=%q, want t1 and t2", series.ID, plain.ID)
	}
	if !series.Series {
		t.Fatalf("series table t1 TableInfo.Series = false, want true: %+v", series)
	}
	if plain.Series {
		t.Fatalf("non-series table t2 TableInfo.Series = true, want false: %+v", plain)
	}
	rawSeries, err := json.Marshal(series)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rawSeries), `"series":true`) {
		t.Fatalf("series table marshalled without series:true: %s", rawSeries)
	}
	rawPlain, err := json.Marshal(plain)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rawPlain), `"series"`) {
		t.Fatalf("non-series table marshalled a series key (omitempty broken): %s", rawPlain)
	}
}

// TestSeriesTableEmitsBetweenGamesThenLive is E3-fx2's second half: a
// subscriber to a series table sees a table_state "between_games" frame while
// the table sideboards between two games, then a table_state "live" frame when
// the next game starts — the signal the E3-9 client had to guess from
// match_end.
func TestSeriesTableEmitsBetweenGamesThenLive(t *testing.T) {
	t.Parallel()
	var s *Session
	var mu sync.Mutex
	var frames []protocol.Frame
	o := testOptions(t)
	// Two planned matches, then stop: the table crosses exactly one game
	// boundary, so between_games must appear between two live frames.
	o.Series = func(ctx context.Context, id TableID, k int) (SeriesGame, bool, error) {
		if k <= 2 {
			return SeriesGame{Decks: []string{"a", "b"}, Chooser: -1}, true, nil
		}
		return SeriesGame{}, false, nil
	}
	// Drain on the match goroutine after each decision's burst,
	// so the subscriber channel never overflows and frame order is preserved.
	o.Sleep = func(time.Duration, <-chan struct{}) {
		mu.Lock()
		drainNonBlocking(s, &frames)
		mu.Unlock()
	}
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(seriesTable("t1")); err != nil {
		t.Fatal(err)
	}
	s = r.OpenSession()
	if err := r.Subscribe(s, "t1", protocol.ModeFocus); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	mu.Lock()
	drainNonBlocking(s, &frames)
	mu.Unlock()

	// Precondition: the table really played the two planned games, so the
	// between_games/live pair below is a real game boundary, not a table
	// that never started.
	ms, err := r.Matches("t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 {
		t.Fatalf("table played %d matches, want 2", len(ms))
	}

	var states []string
	for _, f := range frames {
		if f.T != protocol.TTableState {
			continue
		}
		if f.Table != "t1" {
			t.Fatalf("table_state addressed to %q, want t1", f.Table)
		}
		if f.ID == 0 {
			t.Fatalf("table_state frame has no session id (not resumable)")
		}
		states = append(states, decode[protocol.TableStateBody](t, f).State)
	}
	between := -1
	for i, st := range states {
		if st == protocol.TableBetweenGames {
			between = i
			break
		}
	}
	if between < 0 {
		t.Fatalf("no between_games table_state frame; states=%v", states)
	}
	live := false
	for _, st := range states[between+1:] {
		if st == protocol.TableLive {
			live = true
			break
		}
	}
	if !live {
		t.Fatalf("no live table_state frame after between_games; states=%v", states)
	}
}

// TestNonSeriesTableEmitsNoTableState pins E3-fx2's scope: table_state is a
// series-table signal only. An ordinary table's idle/live/cooldown is already
// carried by match_start/match_end and the overview widget, so emitting
// table_state for it injects an unrequested frame into every non-series
// stream. This is the regression a blanket setState broadcast caused (it made
// a focus stream begin table_state live before match_start, and an overview
// subscriber see between_games), so the test runs a plain one-match table and
// asserts its whole stream carries no table_state at all.
func TestNonSeriesTableEmitsNoTableState(t *testing.T) {
	t.Parallel()
	var s *Session
	var frames []protocol.Frame
	o := testOptions(t)
	o.Sleep = func(time.Duration, <-chan struct{}) { drainNonBlocking(s, &frames) }
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	s = r.OpenSession()
	if err := r.Subscribe(s, "t1", protocol.ModeFocus); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	drainNonBlocking(s, &frames) // pick up match_end, pushed after the last handshake

	// Precondition: the table really played its one match and the stream
	// carried frames, so "no table_state" is a statement about a live stream,
	// not an empty or never-started one.
	ms, err := r.Matches("t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 {
		t.Fatalf("table played %d matches, want 1", len(ms))
	}
	if len(frames) < 100 {
		t.Fatalf("only %d frames; the focus stream never ran", len(frames))
	}
	for _, f := range frames {
		if f.T == protocol.TTableState {
			t.Fatalf("non-series table emitted table_state %q; the frame is series-only", decode[protocol.TableStateBody](t, f).State)
		}
	}
}
