package bench_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/server/matchrecord"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// recordConfig builds a repo-deck game config over the named decks.
func recordConfig(t *testing.T, reg *cards.Registry, seed uint64, names ...string) rules.Config {
	t.Helper()
	decks := make([][]*cards.Card, len(names))
	for i, n := range names {
		d, err := testutil.LoadRepoDeck(reg, n)
		if err != nil {
			t.Fatal(err)
		}
		decks[i] = d
	}
	return rules.Config{Seed: seed, Names: names, Decks: decks, Tokens: reg.Tokens}
}

func heuristicSeats(seed uint64, n int) []seat.Seat {
	seats := make([]seat.Seat, n)
	for i := range seats {
		seats[i] = heuristic.NewBot(seed ^ uint64(i+1))
	}
	return seats
}

// fileSink is the caller-side sink a driver like botbench supplies: one
// file per game under dir.
func fileSink(t *testing.T, dir string, policies []string, paths *[]string) *matchrecord.Sink {
	t.Helper()
	return &matchrecord.Sink{
		Producer: matchrecord.Producer{
			Tool:     "bench-test",
			ForgeRef: matchrecord.ForgeRef(testutil.CorpusDirOrSkip(t)),
			Policies: policies,
			RunID:    "t1",
		},
		Put: func(r *matchrecord.Record) error {
			p := filepath.Join(dir, fmt.Sprintf("g%d%s", len(*paths), matchrecord.Ext))
			*paths = append(*paths, p)
			return matchrecord.Write(p, r)
		},
	}
}

// TestRecordHookRecordsAReplayableGame is T1's Done-means: a 2-seat and a
// 4-seat bench game played with Hooks.Record each write one record, and
// Load then Verify reproduces the live game's head. SyncAnswer is
// requested too and must stay off while Record is set (every answer then
// goes through the drive loop, so the loop's intent count is the log's).
func TestRecordHookRecordsAReplayableGame(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		seed  uint64
		decks []string
	}{
		{424242, []string{"mono-black-aggro", "mono-red-prowess"}},
		{4103, []string{"dimir-tempo", "mono-black-aggro", "uw-control", "mono-red-goblins"}},
	} {
		cfg := recordConfig(t, reg, tc.seed, tc.decks...)
		dir := t.TempDir()
		var paths []string
		pols := make([]string, len(tc.decks))
		for i := range pols {
			pols[i] = "heuristic"
		}
		hooks := bench.Hooks{Record: fileSink(t, dir, pols, &paths), SyncAnswer: true}
		o, e, err := bench.PlayGame(cfg, heuristicSeats(cfg.Seed, len(tc.decks)), 60, 6000, hooks)
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) != 1 {
			t.Fatalf("%d seats: %d records written, want 1", len(tc.decks), len(paths))
		}
		if o.Intents != len(e.L.Intents) {
			t.Errorf("%d seats: the loop counted %d intents, the log holds %d: SyncAnswer answered inline with Record set",
				len(tc.decks), o.Intents, len(e.L.Intents))
		}
		rec, rcfg, err := matchrecord.Load(paths[0], reg)
		if err != nil {
			t.Fatal(err)
		}
		if rec.Head != e.L.Head() {
			t.Fatalf("%d seats: record head %s, live head %s", len(tc.decks), rec.Head, e.L.Head())
		}
		if err := matchrecord.Verify(rec, rcfg); err != nil {
			t.Fatalf("%d seats: %v", len(tc.decks), err)
		}
		if got := strings.Join(rec.Producer.Policies, ","); got != strings.Join(pols, ",") {
			t.Errorf("policies %q", got)
		}
		fi, err := os.Stat(paths[0])
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%d seats: seed %d, %d turns, %d intents, %d events, over=%v stall=%q: record %d B gzipped",
			len(tc.decks), cfg.Seed, e.G.Turn, len(e.L.Intents), len(e.L.Events), e.G.Over, o.StallOn, fi.Size())
	}
}

// TestRecordOfSyncAnsweredGameReplays verifies the claim the Record hook's
// doc leans on rather than trusting it: a game whose mid-resolution asks
// were answered INLINE (SyncAnswer, which bypasses the drive loop) still
// logs every answer as an intent, so a record built from its log replays.
// The game must actually have answered inline, or the test proves nothing.
func TestRecordOfSyncAnsweredGameReplays(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	pairs := [][2]string{
		{"dimir-tempo", "mono-black-aggro"},
		{"foundations-wretched-ranks", "ur-delver"},
		{"uw-control", "mono-red-goblins"},
	}
	for _, p := range pairs {
		for g := uint64(0); g < 6; g++ {
			cfg := recordConfig(t, reg, 4100+g, p[0], p[1])
			o, e, err := bench.PlayGame(cfg, heuristicSeats(cfg.Seed, 2), 60, 4000, bench.Hooks{SyncAnswer: true})
			if err != nil {
				t.Fatal(err)
			}
			inline := len(e.L.Intents) - o.Intents
			if inline <= 0 {
				continue
			}
			rec, err := matchrecord.New(cfg, e.L, matchrecord.Producer{Tool: "bench-test"})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "sync"+matchrecord.Ext)
			if err := matchrecord.Write(path, rec); err != nil {
				t.Fatal(err)
			}
			back, rcfg, err := matchrecord.Load(path, reg)
			if err != nil {
				t.Fatal(err)
			}
			if err := matchrecord.Verify(back, rcfg); err != nil {
				t.Fatalf("%v seed %d (%d inline answers): %v", p, cfg.Seed, inline, err)
			}
			t.Logf("%v seed %d: %d of %d intents answered inline; the record replays", p, cfg.Seed, inline, len(e.L.Intents))
			return
		}
	}
	t.Fatal("no game answered an ask inline; the claim went untested")
}

// TestRecordCarriesNoTokenText pins design D2: a record carries the corpus
// pin, never the GPL-3.0 token script text and never the name-universe
// label list. The record's decompressed bytes may hold neither the
// feedback snapshot's token keys nor any token script's text.
func TestRecordCarriesNoTokenText(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	cfg := recordConfig(t, reg, 7, "mono-red-goblins", "mono-black-aggro")
	cfg.NameUniverse = reg.Universe() // the mode bit must ride, the list must not
	dir := t.TempDir()
	var paths []string
	sink := fileSink(t, dir, []string{"heuristic", "heuristic"}, &paths)
	if _, _, err := bench.PlayGame(cfg, heuristicSeats(cfg.Seed, 2), 60, 4000, bench.Hooks{Record: sink}); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("%d records", len(paths))
	}
	f, err := os.Open(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	var top struct {
		Producer map[string]json.RawMessage `json:"producer"`
		Match    map[string]json.RawMessage `json:"match"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"tokens", "tokens_unread", "name_universe_names"} {
		if _, ok := top.Match[k]; ok {
			t.Errorf("record match carries %q", k)
		}
	}
	if string(top.Match["name_universe"]) != "true" {
		t.Errorf("the name-universe mode bit did not ride the record: %s", top.Match["name_universe"])
	}
	if len(top.Producer["forge_ref"]) <= 2 {
		t.Errorf("record names no forge_ref: %s", top.Producer["forge_ref"])
	}
	// No token script's text: its Name: line is the marker every script
	// starts with, and the whole script must be absent too.
	checked := 0
	for stem, c := range reg.Tokens {
		if c == nil || c.Path == "" {
			continue
		}
		src, err := os.ReadFile(c.Path)
		if err != nil {
			continue
		}
		checked++
		for _, line := range strings.Split(string(src), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Name:") && bytes.Contains(raw, []byte(line)) {
				t.Fatalf("record carries token %s's script line %q", stem, line)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no token script was readable; the check is vacuous")
	}
	if bytes.Contains(raw, []byte("Name:")) {
		t.Error("record carries a script-shaped Name: line")
	}
	rec, rcfg, err := matchrecord.Load(paths[0], reg)
	if err != nil {
		t.Fatal(err)
	}
	if rcfg.NameUniverse == nil || rcfg.NameUniverseNames != nil {
		t.Errorf("Load: universe %v, names %d: want the corpus universe and a derived list", rcfg.NameUniverse != nil, len(rcfg.NameUniverseNames))
	}
	if err := matchrecord.Verify(rec, rcfg); err != nil {
		t.Fatal(err)
	}
}
