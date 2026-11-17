package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/cardcov"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

type covRow struct {
	seed, card, key        string
	used, offered, cleanOK string
}

// readCoverage parses a -coverage file: its header lines and its rows.
func readCoverage(t *testing.T, path string) (header []string, rows []covRow) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading coverage file: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("coverage file has %d lines, want a header, a column line and rows", len(lines))
	}
	header = lines[:2]
	for _, l := range lines[2:] {
		f := strings.Split(l, "\t")
		if len(f) != 6 {
			t.Fatalf("row %q has %d fields, want 6", l, len(f))
		}
		rows = append(rows, covRow{f[0], f[1], f[2], f[3], f[4], f[5]})
	}
	return header, rows
}

// TestCoverageRows: a seeded 2-seat repo-deck game credits a deck row per
// distinct dealt card and a used cast row for every card the log shows put on
// the stack, with clean=1; a game that fails writes clean=0.
func TestCoverageRows(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	path := filepath.Join(t.TempDir(), "cov.tsv")
	var out bytes.Buffer
	o := simOptions{Seats: 2, Seed: 3, Games: 1, Verify: true, Dir: dir, Format: "constructed", Coverage: path, CoverageCtx: "c60"}
	if err := runSim(o, &out); err != nil {
		t.Fatalf("runSim: %v\n%s", err, out.String())
	}
	header, rows := readCoverage(t, path)
	if !strings.HasPrefix(header[0], "#cov-1\tcommit=") || !strings.Contains(header[0], "\tctx=c60\tformat=constructed\tseats=2\tcorpus=") {
		t.Errorf("header = %q", header[0])
	}
	if header[1] != "seed\tcard\tkey\tused\toffered\tclean" {
		t.Errorf("column line = %q", header[1])
	}

	// Replay the same game independently for its event log and deck.
	reg, err := testutil.OpenCorpusRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := testutil.LegacyDeckNames()[:2]
	decks := make([][]*cards.Card, 2)
	dealt := map[string]bool{}
	for i, n := range names {
		if decks[i], err = testutil.LoadRepoDeck(reg, n); err != nil {
			t.Fatal(err)
		}
		for _, c := range decks[i] {
			dealt[c.Faces[0].Name] = true
		}
	}
	cfg := gameConfig(3, names, decks, reg, rules.FormatConstructed, nil, 0)
	cfg.LoopGuard = rules.WithDefaultObjectCap(cfg.LoopGuard)
	e := rules.NewStartingPlayerChoice(cfg)
	e.AskStartingPlayer()
	seats, err := matchSeats(e, reg, 3, names, make([]string, 2), "constructed", 0, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := driveMatch(e, seats, newSeatRoute(cfg, seats), nil); err != nil {
		t.Fatal(err)
	}
	cast := map[string]bool{}
	for _, ev := range e.L.Events {
		if ev.Kind == events.PutOnStack && ev.To == state.ZStack {
			if n := cardcov.RealCardName(e, ev.Obj); n != "" {
				cast[n] = true
			}
		}
	}
	if len(cast) == 0 {
		t.Fatal("precondition: the game cast nothing, so the cast assertion is vacuous")
	}

	deckRows, castRows := map[string]covRow{}, map[string]covRow{}
	var order []string
	for _, r := range rows {
		if r.seed != "3" || r.cleanOK != "1" {
			t.Errorf("row %+v: want seed 3 and clean=1", r)
		}
		if r.used != "0" && r.used != "1" || r.offered != "0" && r.offered != "1" {
			t.Errorf("row %+v: used/offered must be 0/1", r)
		}
		switch r.key {
		case "deck":
			deckRows[r.card] = r
		case "cast", "land":
			castRows[r.card] = r
		}
		order = append(order, r.card+"\t"+r.key)
	}
	if !sort.StringsAreSorted(order) {
		t.Error("rows are not in sorted (card, key) order")
	}
	if len(deckRows) != len(dealt) {
		t.Errorf("%d deck rows, want %d distinct dealt cards", len(deckRows), len(dealt))
	}
	for n := range dealt {
		if r, ok := deckRows[n]; !ok || r.used != "1" {
			t.Errorf("deck row for %q = %+v, want used=1", n, r)
		}
	}
	for n := range cast {
		if r, ok := castRows[n]; !ok || r.used != "1" {
			t.Errorf("cast row for %q = %+v, want used=1", n, r)
		}
	}

	// A game that fails writes clean=0 (and the file keeps one header).
	// Force playOne's failure without registering a test bot: a registered
	// failing policy would join bots.Names() and TestRunPlaysEveryKnownPolicy
	// plays every entry there. An unknown policy makes matchSeats refuse the
	// seat before the first decision, which is playOne's false return.
	cov, err := openCoverage(path, "c60", "constructed", 2, dir, reg)
	if err != nil {
		t.Fatalf("openCoverage: %v", err)
	}
	var failedOut bytes.Buffer
	if playOne(&failedOut, 4, names, decks, reg, false, []string{"coverage-test-no-such-policy"}, rules.FormatConstructed, nil, 0, nil, cov) {
		t.Fatal("forced failure: playOne returned ok")
	}
	if !strings.Contains(failedOut.String(), "FAILED") {
		t.Errorf("forced failure output = %q, want a FAILED line", failedOut.String())
	}
	if err := cov.f.Close(); err != nil {
		t.Fatalf("closing coverage file: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if n := strings.Count(string(raw), "#cov-1"); n != 1 {
		t.Errorf("%d headers after an append, want 1", n)
	}
	_, rows = readCoverage(t, path)
	failed := 0
	for _, r := range rows {
		if r.seed == "4" {
			failed++
			if r.cleanOK != "0" {
				t.Errorf("failed game row %+v: want clean=0", r)
			}
		}
	}
	if failed == 0 {
		t.Error("the failed game wrote no rows")
	}
}

// TestCoverageListKeys: -list-keys prints card<TAB>key for every card in name
// order: cast or land, then each Keys and ModeKeys key.
func TestCoverageListKeys(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	var buf bytes.Buffer
	if err := listKeys(dir, &buf); err != nil {
		t.Fatalf("listKeys: %v", err)
	}
	reg, err := testutil.OpenCorpusRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	all := append([]*cards.Card(nil), reg.AllCards()...)
	sort.SliceStable(all, func(i, j int) bool { return all[i].Faces[0].Name < all[j].Faces[0].Name })
	for _, c := range all {
		n := c.Faces[0].Name
		k := "cast"
		if c.Faces[0].IsLand() {
			k = "land"
		}
		want = append(want, n+"\t"+k)
		for _, key := range cardcov.Keys(c) {
			want = append(want, n+"\t"+key)
		}
		for _, key := range cardcov.ModeKeys(c) {
			want = append(want, n+"\t"+key)
		}
	}
	got := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(got) != len(want) || len(want) < 1000 {
		t.Fatalf("%d lines, want %d (and a real corpus)", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
	for _, must := range []string{"Lightning Bolt\tcast", "Island\tland"} {
		if !strings.Contains(buf.String(), "\n"+must+"\n") && !strings.HasPrefix(buf.String(), must+"\n") {
			t.Errorf("missing %q", must)
		}
	}
}
