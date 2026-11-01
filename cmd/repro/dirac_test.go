package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
	"github.com/adams-shaun/gorge/testkit/testutil/feedback"
)

// diracCapture plays two bench decks on dirac, bot against bot, and captures the
// finished match the way a report does.
func diracCapture(t *testing.T) string {
	t.Helper()
	root, err := feedback.Root()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	reg := testutil.CorpusRegistry(t)
	e, err := adapter.New([]string{"fdn"}, reg, dirac.SurfaceFull)
	if err != nil {
		t.Skip(err)
	}
	decks := map[string][]*cards.Card{}
	names := []string{"000_FDN_top_00001_WB", "001_FDN_top_00032_WU"}
	for _, n := range names {
		_, d, err := deck.Load(reg, filepath.Join(root, "dirac/testdata/bench/fdn", n+".json"))
		if err != nil {
			t.Fatal(err)
		}
		decks[n] = d
	}
	r, err := host.New(host.Options{
		LoadDeck: func(name string) (host.Deck, error) {
			if d, ok := decks[name]; ok {
				return host.Deck{Name: name, Cards: d}, nil
			}
			return host.Deck{}, host.ErrNotFound
		},
		Tokens:  reg.Tokens,
		Engines: map[string]game.Engine{adapter.Name: e, gorgegame.Name: gorgegame.Engine{}},
		Seats: func(_ []string, seed uint64) []seat.Seat {
			return []seat.Seat{heuristic.NewBot(seed ^ 1), heuristic.NewBot(seed ^ 2)}
		},
		Sleep: func(time.Duration, <-chan struct{}) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	if err := r.AddTable(host.TableConfig{ID: "t1", Name: "t1", Seats: 2, Decks: names, Seed: 42, Spectator: view.Public, AutoMana: true, Engine: adapter.Name}); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(40 * time.Second)
	for {
		ms, err := r.Matches("t1")
		if err != nil {
			t.Fatal(err)
		}
		if len(ms) > 0 && ms[0].State != protocol.MatchLive {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no finished match")
		}
		time.Sleep(time.Second / 100)
	}
	seatOne := state.PlayerID(1)
	snap, err := r.SnapshotMatch("t1", 1, &seatOne)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "report")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string]any{"match.json": snap.Match, "log.json": snap.Log, "view.json": snap.View} {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func reproRun(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var out, errs bytes.Buffer
	code := run(args, &out, &errs)
	return code, out.String() + errs.String()
}

func TestDiracCaptureReplays(t *testing.T) {
	dir := diracCapture(t)
	if code, out := reproRun(t, dir); code != 0 || !strings.Contains(out, "game over") || !strings.Contains(out, "last log lines") {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	if code, out := reproRun(t, dir, "-at", "10"); code != 0 || !strings.Contains(out, "replayed 10 of") || !strings.Contains(out, "pending: seat") {
		t.Fatalf("-at: exit %d:\n%s", code, out)
	}
	if code, out := reproRun(t, dir, "-list"); code != 0 || !strings.Contains(out, "game over") {
		t.Fatalf("-list: exit %d:\n%s", code, out)
	}
	code, out := reproRun(t, dir, "-omniscient", "-at", "1")
	if code != 0 {
		t.Fatalf("-omniscient: exit %d:\n%s", code, out)
	}
	if g := feedback.GameAt(t, dir, 10); g.Pending() == nil {
		t.Fatal("GameAt: no decision pending at intent 10")
	}
}

// TestDiracCaptureHoldsNoScripts: a dirac capture has no token text to strip.
func TestDiracCaptureHoldsNoScripts(t *testing.T) {
	dir := diracCapture(t)
	for _, f := range []string{"match.json", "log.json", "view.json"} {
		raw, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte("Oracle:")) {
			t.Errorf("%s holds script text", f)
		}
	}
}

func TestReproLockstepAgrees(t *testing.T) {
	dir := diracCapture(t)
	code, out := reproRun(t, dir, "-lockstep")
	if code != 0 || !strings.Contains(out, "gorge agrees") {
		t.Fatalf("exit %d:\n%s", code, out)
	}
}

func TestTableKeyMismatchSaysSo(t *testing.T) {
	dir := diracCapture(t)
	path := filepath.Join(dir, "match.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	doc["engine_key"] = fmt.Sprint(doc["engine_key"]) + "0"
	raw, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := reproRun(t, dir)
	if code != 1 || !strings.Contains(out, "DIVERGED") || !strings.Contains(out, "tables moved") {
		t.Fatalf("exit %d:\n%s", code, out)
	}
}

func TestLockstepRefusesAGorgeCapture(t *testing.T) {
	var out, errs bytes.Buffer
	dir := filepath.Join("testdata", "feedback", fixtureID)
	if code := run([]string{dir, "-lockstep"}, &out, &errs); code != 2 || !strings.Contains(errs.String(), "dirac capture") {
		t.Fatalf("exit %d: %s%s", code, out.String(), errs.String())
	}
}
