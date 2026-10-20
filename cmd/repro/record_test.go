package main

import (
	"bytes"
	"io"
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

// benchRecord plays one 2-seat bench game with Hooks.Record and returns the
// record file's path. Generated in-test: a record holds every hidden card
// and is never committed.
func benchRecord(t *testing.T) string {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	names := []string{"mono-black-aggro", "mono-red-prowess"}
	decks := make([][]*cards.Card, len(names))
	for i, n := range names {
		d, err := testutil.LoadRepoDeck(reg, n)
		if err != nil {
			t.Fatal(err)
		}
		decks[i] = d
	}
	cfg := rules.Config{Seed: 424242, Names: names, Decks: decks, Tokens: reg.Tokens}
	path := filepath.Join(t.TempDir(), "bench-424242"+matchrecord.Ext)
	sink := &matchrecord.Sink{
		Producer: matchrecord.Producer{
			Tool: "repro-test", ForgeRef: matchrecord.ForgeRef(testutil.CorpusDirOrSkip(t)),
			Policies: []string{"heuristic", "heuristic"},
		},
		Put: func(r *matchrecord.Record) error { return matchrecord.Write(path, r) },
	}
	seats := []seat.Seat{heuristic.NewBot(cfg.Seed ^ 1), heuristic.NewBot(cfg.Seed ^ 2)}
	if _, _, err := bench.PlayGame(cfg, seats, 60, 4000, bench.Hooks{Record: sink}); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestReproAcceptsAMatchRecord: a match record file works everywhere a
// feedback directory does (the summary, -at, -list, -omniscient), and
// -emit-test refuses it.
func TestReproAcceptsAMatchRecord(t *testing.T) {
	path := benchRecord(t)
	for _, tc := range []struct {
		args []string
		want []string
	}{
		{[]string{path}, []string{"== match record bench-424242 (tool repro-test, policies heuristic v heuristic", "replayed", "game over"}},
		{[]string{path, "-at", "10"}, []string{"replayed 10 of"}},
		{[]string{path, "-list"}, []string{"   0  seat ", "game over"}},
		{[]string{"-omniscient", path, "-at", "20"}, []string{"  hand: "}},
	} {
		var out, errOut bytes.Buffer
		if code := run(tc.args, &out, &errOut); code != 0 {
			t.Fatalf("%v: exit %d\n%s%s", tc.args, code, out.String(), errOut.String())
		}
		for _, w := range tc.want {
			if !strings.Contains(out.String(), w) {
				t.Errorf("%v: output missing %q:\n%s", tc.args, w, out.String())
			}
		}
		if strings.Contains(out.String(), "warning: the record was played against corpus") {
			t.Errorf("%v: same-corpus record warned about its pin:\n%s", tc.args, out.String())
		}
	}
	var errOut bytes.Buffer
	if code := run([]string{path, "-emit-test", "engine/rules"}, io.Discard, &errOut); code != 2 ||
		!strings.Contains(errOut.String(), "never committed") {
		t.Fatalf("-emit-test on a record: exit %d: %s", code, errOut.String())
	}
}
