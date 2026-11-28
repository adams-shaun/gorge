package host

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/protocol"
)

// keepMatchCooldown marks the between-matches Sleep call for the retention
// tests below: Sleep is also called after every decision with the table's
// Pace (0 here), so the value is only a discriminator.
const keepMatchCooldown = time.Hour

// playBotMatches plays n matches on cfg in persistence mode and closes the
// registry once the nth between-matches cooldown starts, so exactly n
// matches have been archived.
func playBotMatches(t *testing.T, o Options, cfg TableConfig, n int) *Registry {
	t.Helper()
	o.Cooldown = keepMatchCooldown
	var (
		r    *Registry
		seen int
	)
	o.Sleep = func(d time.Duration, stop <-chan struct{}) {
		if d != keepMatchCooldown {
			return
		}
		seen++
		if seen == n {
			go r.Close()
			<-r.Done()
		}
	}
	var err error
	r, err = New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(cfg.ID); err != nil {
		t.Fatal(err)
	}
	r.Wait(cfg.ID)
	if seen < n {
		t.Fatalf("only %d matches finished, want %d", seen, n)
	}
	return r
}

// matchFilesIn counts the <k>.json/<k>.events/<k>.intents files left in one
// table's directory.
func matchFilesIn(t *testing.T, dir string, id TableID) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, string(id)))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		switch filepath.Ext(e.Name()) {
		case ".json", ".events", ".intents":
			n++
		}
	}
	return n
}

// TestKeepMatchesPrunesOldBotMatches: a bot-only perpetual table with
// KeepMatches 3 keeps the newest three matches on disk and in the match
// list, while the counter keeps counting so a pruned number is never reused.
func TestKeepMatchesPrunesOldBotMatches(t *testing.T) {
	const keep, played = 3, 8
	dir := t.TempDir()
	o := diskOptions(t, dir)
	o.KeepMatches = keep
	r := playBotMatches(t, o, twoSeatPerpetual("t1"), played)

	ms, err := r.Matches("t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != keep {
		t.Fatalf("Matches lists %d matches, want %d: %+v", len(ms), keep, ms)
	}
	for i, m := range ms {
		if want := played - keep + 1 + i; m.Match != want {
			t.Fatalf("Matches[%d] is match %d, want %d", i, m.Match, want)
		}
	}
	// The table's counter did not rewind with the pruned entries.
	if ti := r.Tables(); len(ti) != 1 || ti[0].Match != played {
		t.Fatalf("table info after %d matches: %+v", played, ti)
	}
	if n, want := matchFilesIn(t, dir, "t1"), keep*3; n != want {
		t.Fatalf("table directory holds %d match files, want %d", n, want)
	}
	if _, err := r.ViewAt("t1", 1, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ViewAt of pruned match 1: err = %v, want ErrNotFound", err)
	}
}

// TestKeepMatchesNeverDeletesHumanOrCrashed pins the prunable rule: only a
// finished/aborted all-bot match of a bot-only, non-event table may go. A
// table configured with a human seat keeps all its matches whatever N is,
// even when the stored sidecars themselves are all-bot.
func TestKeepMatchesNeverDeletesHumanOrCrashed(t *testing.T) {
	botTable := TableConfig{Seats: 2, Decks: []string{"a", "b"}}
	botSeats := []protocol.SeatInfo{{Name: "a", Deck: "a"}, {Name: "b", Deck: "b"}}
	humanSeats := []protocol.SeatInfo{{Name: "a", Deck: "a", Human: true}, {Name: "b", Deck: "b"}}
	eventTable := botTable
	eventTable.Event = "draft-1"
	cases := []struct {
		name string
		cfg  TableConfig
		sc   sidecar
		want bool
	}{
		{"bot finished", botTable, sidecar{State: protocol.MatchFinished, Seats: botSeats}, true},
		{"human seat", botTable, sidecar{State: protocol.MatchFinished, Seats: humanSeats}, false},
		{"crashed", botTable, sidecar{State: protocol.MatchCrashed, Seats: botSeats}, false},
		{"event table", eventTable, sidecar{State: protocol.MatchFinished, Seats: botSeats}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := prunable(tc.cfg, tc.sc); got != tc.want {
				t.Fatalf("prunable(%+v, %+v) = %v, want %v", tc.cfg, tc.sc, got, tc.want)
			}
		})
	}

	// A registered human table keeps every archived match at KeepMatches 1.
	dir := t.TempDir()
	r, err := New(diskOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	cfg := twoSeatPerpetual("t1")
	cfg.Perpetual = false
	cfg.Humans = []int{0}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	for k := 1; k <= 3; k++ {
		sc := sidecar{Table: "t1", Match: k, State: protocol.MatchFinished, Seats: botSeats, Decks: []string{"a", "b"}}
		if err := writeSidecar(dir, sc, false); err != nil {
			t.Fatal(err)
		}
		for _, ext := range []string{".events", ".intents"} {
			if err := os.WriteFile(matchPath(dir, "t1", k, ext), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	o := diskOptions(t, dir)
	o.KeepMatches = 1
	r2, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Close()
	ms, err := r2.Matches("t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 3 {
		t.Fatalf("human table kept %d matches at KeepMatches 1, want all 3", len(ms))
	}
	if n, want := matchFilesIn(t, dir, "t1"), 9; n != want {
		t.Fatalf("human table directory holds %d match files, want %d", n, want)
	}
}

// TestKeepMatchesPrunesAtRestart: pruning also runs when a directory is
// reopened, so an existing bot table shrinks to N retained matches.
func TestKeepMatchesPrunesAtRestart(t *testing.T) {
	const played = 6
	dir := t.TempDir()
	playBotMatches(t, diskOptions(t, dir), twoSeatPerpetual("t1"), played)

	o := diskOptions(t, dir)
	o.KeepMatches = 2
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ms, err := r.Matches("t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 || ms[0].Match != 5 || ms[1].Match != 6 {
		t.Fatalf("after restart Matches = %+v, want matches 5 and 6", ms)
	}
	if n, want := matchFilesIn(t, dir, "t1"), 6; n != want {
		t.Fatalf("table directory holds %d match files after restart, want %d", n, want)
	}
	if _, err := r.ViewAt("t1", 1, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ViewAt of pruned match 1 after restart: err = %v, want ErrNotFound", err)
	}
}
