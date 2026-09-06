package host

// Task M2e-5: making the London mulligan reachable from the product, without
// breaking replay. The whole hazard is R-8.4: Config.Mulligans must travel
// in the same Config value replay is handed, and the replay Config for a
// finished match is rebuilt from the persisted sidecar (host/viewat.go), not
// from the live TableConfig — so the value must persist, and a sidecar that
// loses it silently replays a different game.
//
// TestAMulliganTableReplaysExactly is the pair to watch: it replays through
// the host's own replay build, so a replay Config that drops the value fails
// here directly. TestAMulliganTableSurvivesARestart is the sidecar
// round-trip: a fresh registry knows a match only from disk, so a sidecar
// field that was never written loads as 0 and the rebuilt match cannot
// replay. TestAnOldSidecarWithoutMulligansLoadsAsZero pins R-E5-2 (no key ≡
// 0, nothing migrated), and TestMulligansZeroIsUnchanged pins R-E5-1's other
// half: an explicit 0 is byte-identical to no field at all.

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

// mulliganTable is a two-seat table with the London round switched on: each
// player may take up to n mulligans between the deal and turn 1. Two seats
// is the smallest shape where a mulligan round exists at all, and the shape
// the task's own product verification played (gorged -seats 2); the replay
// pair it feeds is the same pair a four-seat table would need.
func mulliganTable(id TableID, n int) TableConfig {
	return TableConfig{ID: id, Name: "Table " + string(id), Seats: 2, Decks: []string{"a", "b"},
		Seed: 99, Pace: 0, Spectator: view.Omniscient, Perpetual: false, Mulligans: n}
}

// TestAMulliganTableReplaysExactly is the task's one that matters: a table
// with Mulligans: 1 plays a match to completion, and the match replays from
// its log to the same chain head through the host's own replay build
// (matchForLog, host/viewat.go:261) — not a test-side config — so a replay
// Config that loses the value fails right here. The sidecar is asserted
// explicitly too: it is the value the replay must read back through, and
// the one site where a dropped write silently reads as 0.
func TestAMulliganTableReplaysExactly(t *testing.T) {
	t.Parallel()
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(mulliganTable("t1", 1)); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")

	r.mu.RLock()
	tb := r.tables["t1"]
	r.mu.RUnlock()
	tb.mu.RLock()
	m := tb.history[0]
	tb.mu.RUnlock()
	if m == nil || m.state != protocol.MatchFinished {
		t.Fatalf("mulligan table did not finish: %+v", m)
	}
	liveHead := m.e.L.Head()

	// The sidecar the host writes — m.sidecar() is the production writer
	// (match start and archive both go through it), so the replay reads the
	// same value a restart would have on disk.
	sc := m.sidecar()
	if sc.Mulligans != 1 {
		t.Fatalf("sidecar lost the mulligan count: %d, want 1", sc.Mulligans)
	}

	// The host's own replay build: config from the sidecar, log from the
	// match. A Config that drops Mulligans re-runs without the round, the
	// first recorded intent (a mulligan ask) is refused against the turn-1
	// decision, and Replay errors.
	sm, err := r.matchForLog(tb, sc, m.e.L)
	if err != nil {
		t.Fatalf("mulligan match does not replay: %v", err)
	}
	if got, want := sm.e.L.Head(), liveHead; got != want {
		t.Fatalf("mulligan replay head %s, live head %s", got, want)
	}
}

// TestAMulliganTableSurvivesARestart writes the sidecar, reloads it with a
// fresh registry (the restart), and replays from the reloaded state. The
// second registry knows the match only from disk, so a sidecar field that
// was never written loads as 0, the rebuilt config skips the round, and the
// first logged mulligan intent is refused the moment Events (or ViewAt)
// forces the replay — the exact failure this task is here to prevent.
func TestAMulliganTableSurvivesARestart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var liveHead string
	var liveEvents int
	{
		r, err := New(diskOptions(t, dir))
		if err != nil {
			t.Fatal(err)
		}
		if err := r.AddTable(mulliganTable("t1", 1)); err != nil {
			t.Fatal(err)
		}
		_ = r.Start("t1")
		r.Wait("t1")
		ms, err := r.Matches("t1")
		if err != nil || len(ms) != 1 || ms[0].State != protocol.MatchFinished {
			t.Fatalf("matches before restart: %+v, %v", ms, err)
		}
		liveHead, liveEvents = ms[0].Head, ms[0].Events
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
	}

	r2, err := New(diskOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Close()
	ms, err := r2.Matches("t1")
	if err != nil || len(ms) != 1 || ms[0].State != protocol.MatchFinished {
		t.Fatalf("after restart: %+v, %v", ms, err)
	}
	if ms[0].Head != liveHead || ms[0].Events != liveEvents {
		t.Fatalf("after restart: head %s/%d events, live was %s/%d", ms[0].Head, ms[0].Events, liveHead, liveEvents)
	}
	// Events drives loadArchived -> matchForLog -> replay.Replay; with the
	// sidecar value lost the replay diverges and this errors.
	evs, err := r2.Events("t1", 1, 0)
	if err != nil {
		t.Fatalf("restarted mulligan match does not replay: %v", err)
	}
	if len(evs) != liveEvents {
		t.Fatalf("restarted replay serves %d events, want %d", len(evs), liveEvents)
	}
	v, err := r2.ViewAt("t1", 1, uint64(liveEvents-1))
	if err != nil || !v.Over {
		t.Fatalf("restarted head view: over=%v, %v", v.Over, err)
	}
}

// TestAnOldSidecarWithoutMulligansLoadsAsZero is R-E5-2: a sidecar written
// before this task has no "mulligans" key and must still load — as 0, which
// is exactly what those matches played with, so their replays stay correct.
// The fixture is the pre-task file shape (no key, anywhere); readSidecar is
// the real read path, and the rest of the fields must round-trip untouched.
func TestAnOldSidecarWithoutMulligansLoadsAsZero(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "t1"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := `{
  "table": "t1",
  "match": 1,
  "seed": 7,
  "names": ["a", "b"],
  "decks": ["a", "b"],
  "spectator": "omniscient",
  "state": "finished",
  "result": "win",
  "events": 431,
  "turns": 12
}
`
	if err := os.WriteFile(filepath.Join(dir, "t1", "1.json"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	sc, err := readSidecar(dir, "t1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if sc.Mulligans != 0 {
		t.Fatalf("old sidecar loaded mulligans %d, want 0 (that match played with no round)", sc.Mulligans)
	}
	if sc.Seed != 7 || sc.Events != 431 || sc.Turns != 12 || sc.State != "finished" {
		t.Fatalf("old sidecar fields mangled by the new field: %+v", sc)
	}
}

// TestMulligansZeroIsUnchanged pins R-E5-1's other half: a table with
// Mulligans set to 0 produces a byte-identical log to one built without the
// field at all. The no-field half is the shared fixture (played once per
// test binary by fourSeatTable("t1", false), seed 99); the explicit-zero
// half is played here under the same configuration, and the two must agree
// event for event, intent for intent, and on the chain head.
func TestMulligansZeroIsUnchanged(t *testing.T) {
	t.Parallel()
	_, shared := finishedTable(t)

	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	c := fourSeatTable("t1", false)
	c.Mulligans = 0
	if err := r.AddTable(c); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	r.mu.RLock()
	tb := r.tables["t1"]
	r.mu.RUnlock()
	tb.mu.RLock()
	m := tb.history[0]
	tb.mu.RUnlock()
	if m == nil || m.state != protocol.MatchFinished {
		t.Fatalf("zero-mulligan table did not finish: %+v", m)
	}

	if !reflect.DeepEqual(m.e.L.Events, shared.e.L.Events) {
		t.Fatal("Mulligans: 0 changed the event stream vs a config without the field")
	}
	if !reflect.DeepEqual(m.e.L.Intents, shared.e.L.Intents) {
		t.Fatal("Mulligans: 0 changed the intent stream vs a config without the field")
	}
	if got, want := m.e.L.Head(), shared.e.L.Head(); got != want {
		t.Fatalf("Mulligans: 0 log head %s, no-field head %s", got, want)
	}
}
