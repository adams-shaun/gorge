package main

import (
	"fmt"
	"strings"
	"testing"
)

// parseTallyLine parses one gauntlet summary line ("built: 48-30-8  win%
// 0.558  ci95 [0.479, 0.635]") into its W-L-D, win proportion and CI; the
// pooled lines are the stable contract the tests assert on.
func parseTallyLine(t *testing.T, line string) (w, l, d int, p, lo, hi float64) {
	t.Helper()
	f := strings.Fields(line)
	// The label spans one field ("built:") or three ("built seat 0:"); the
	// W-L-D cell is the token just before the "win%" marker.
	wi := -1
	for i, tok := range f {
		if tok == "win%" {
			wi = i
			break
		}
	}
	if wi < 2 || len(f) < wi+5 || f[wi+2] != "ci95" {
		t.Fatalf("summary line %q does not match the W-L-D win%% ci95 shape", line)
	}
	if _, err := fmt.Sscanf(f[wi-1], "%d-%d-%d", &w, &l, &d); err != nil {
		t.Fatalf("summary line %q W-L-D: %v", line, err)
	}
	if _, err := fmt.Sscanf(f[wi+1], "%f", &p); err != nil {
		t.Fatalf("summary line %q win%%: %v", line, err)
	}
	if _, err := fmt.Sscanf(strings.Trim(f[wi+3], "["), "%f", &lo); err != nil {
		t.Fatalf("summary line %q ci95 low: %v", line, err)
	}
	if _, err := fmt.Sscanf(strings.Trim(f[wi+4], "]"), "%f", &hi); err != nil {
		t.Fatalf("summary line %q ci95 high: %v", line, err)
	}
	return w, l, d, p, lo, hi
}

// pooledTally finds the pooled line for label ("built:", "random:",
// "sanity:") among the summary lines and parses it.
func pooledTally(t *testing.T, out, label string) (w, l, d int, p, lo, hi float64) {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), label) {
			continue
		}
		// The per-seat lines embed the label too ("built seat 0:"), so the
		// pooled line is the one whose FIRST field is the bare label.
		if fields := strings.Fields(line); len(fields) > 0 && fields[0] == label {
			return parseTallyLine(t, strings.TrimSpace(line))
		}
	}
	t.Fatalf("no pooled %q summary line in output:\n%s", label, out)
	return 0, 0, 0, 0, 0, 0
}

// countSeatLines counts the per-seat summary lines for a side and checks
// each one's game count and that the win proportion sits inside its own
// Wilson interval (a mathematical precondition of the CI, so a broken
// formatter cannot pass silently).
func countSeatLines(t *testing.T, out, side string, wantSeats, wantGamesEach int) {
	t.Helper()
	seen := 0
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), side+" seat ") {
			continue
		}
		seen++
		w, l, d, p, lo, hi := parseTallyLine(t, strings.TrimSpace(line))
		if w+l+d != wantGamesEach {
			t.Fatalf("%s line %q counts %d games, want %d", side, line, w+l+d, wantGamesEach)
		}
		if p < lo || p > hi {
			t.Fatalf("%s line %q win%% %.3f sits outside its Wilson interval [%.3f, %.3f]", side, line, p, lo, hi)
		}
	}
	if seen != wantSeats {
		t.Fatalf("output carries %d %s per-seat lines, want %d:\n%s", seen, side, wantSeats, out)
	}
}

// TestGauntletSummaryLinesParseAndCount is E0-7's exit gate: one FDN event
// with the gauntlet on, one game per pairing, against a 2-deck human subset
// (the internal test cap), with the random baseline. It asserts the event
// itself ran (the gauntlet's input decks exist), that the pooled built,
// random and sanity lines parse and their counts add up to the pairings
// actually played, and that the per-seat lines each carry their share.
func TestGauntletSummaryLinesParseAndCount(t *testing.T) {
	dir := corpusReady(t)
	o := options{
		set: "FDN", seed: 1, events: 1, players: "bot,bot,bot,bot",
		drafter: "rating-colour", builder: "best23", dir: dir,
		gauntlet:         "fdn-limited",
		gauntletGames:    1,
		gauntletPlayer:   "bot",
		baseline:         "random",
		gauntletMaxHuman: 2,
	}
	var out strings.Builder
	res, err := run(o, &out)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.events) != 1 {
		t.Fatalf("run produced %d events, want 1", len(res.events))
	}
	er := res.events[0]

	// Precondition: the event's own draft pipeline ran and built four
	// 40-card decks — the gauntlet's inputs. 6 round-robin games too.
	if len(er.games) != 6 {
		t.Fatalf("event played %d round-robin games, want 6", len(er.games))
	}
	if len(er.decks) != 4 {
		t.Fatalf("event saved %d built decks, want 4", len(er.decks))
	}
	for s, d := range er.decks {
		if len(d) != 40 {
			t.Fatalf("seat %d's built deck has %d cards, want 40", s, len(d))
		}
	}

	// With 2 human decks and 1 game per pairing: every seat's built deck
	// plays 2 games (pooled 8), the random baseline the same, and the 2
	// human mirror pairings play 1 sanity game each (pooled 2).
	bw, bl, bd, p, lo, hi := pooledTally(t, out.String(), "built:")
	if bw+bl+bd != 8 {
		t.Fatalf("pooled built line counts %d games, want 8 (4 seats × 2 human decks × 1 game):\n%s", bw+bl+bd, out.String())
	}
	if p < lo || p > hi {
		t.Fatalf("pooled built win%% %.3f outside its Wilson interval [%.3f, %.3f]", p, lo, hi)
	}
	rw, rl, rd, _, _, _ := pooledTally(t, out.String(), "random:")
	if rw+rl+rd != 8 {
		t.Fatalf("pooled random line counts %d games, want 8:\n%s", rw+rl+rd, out.String())
	}
	sw, sl, sd, _, _, _ := pooledTally(t, out.String(), "sanity:")
	if sw+sl+sd != 2 {
		t.Fatalf("pooled sanity line counts %d games, want 2 (2 human mirror pairings × 1 game):\n%s", sw+sl+sd, out.String())
	}

	// The per-seat lines: 4 built, 4 random, 2 games each.
	countSeatLines(t, out.String(), "built", 4, 2)
	countSeatLines(t, out.String(), "random", 4, 2)
}

// TestGauntletVerifyReplaysChains pins -verify compatibility: with verify
// on, every gauntlet game's chain is replayed inside run(), so a diverging
// measurement game fails the run. One human deck, one game per pairing,
// baseline none keeps it inside the budget while still covering all three
// game kinds' plumbing (built pairings plus one sanity mirror).
func TestGauntletVerifyReplaysChains(t *testing.T) {
	dir := corpusReady(t)
	o := options{
		set: "FDN", seed: 1, events: 1, players: "bot,bot,bot,bot",
		drafter: "rating-colour", builder: "best23", dir: dir, verify: true,
		gauntlet:         "fdn-limited",
		gauntletGames:    1,
		gauntletPlayer:   "bot",
		baseline:         "none",
		gauntletMaxHuman: 1,
	}
	var out strings.Builder
	res, err := run(o, &out)
	if err != nil {
		t.Fatalf("run with -verify: %v", err)
	}
	if len(res.events) != 1 {
		t.Fatalf("run produced %d events, want 1", len(res.events))
	}
	// Precondition: the gauntlet really played — 4 seats × 1 human deck ×
	// 1 game = 4 built games, plus the one sanity mirror — and the pooled
	// built line reports them.
	bw, bl, bd, _, _, _ := pooledTally(t, out.String(), "built:")
	if bw+bl+bd != 4 {
		t.Fatalf("pooled built line counts %d games, want 4:\n%s", bw+bl+bd, out.String())
	}
	sw, sl, sd, _, _, _ := pooledTally(t, out.String(), "sanity:")
	if sw+sl+sd != 1 {
		t.Fatalf("pooled sanity line counts %d games, want 1:\n%s", sw+sl+sd, out.String())
	}
	if strings.Contains(out.String(), "\n  random:") {
		t.Fatalf("baseline none printed a random line:\n%s", out.String())
	}
}

// TestGauntletRefusals pins the -gauntlet validation: the unknown catalog
// value, a non-FDN set, a bad baseline, fewer than one game per pairing
// and an Env policy on the measurement seats are all refused before
// anything runs (dir "." — the refusals must not need the corpus).
func TestGauntletRefusals(t *testing.T) {
	base := options{
		set: "FDN", seed: 1, events: 1, players: "bot,bot,bot,bot",
		drafter: "rating-colour", builder: "best23", dir: ".",
		gauntlet: "fdn-limited", gauntletGames: 1, gauntletPlayer: "bot", baseline: "random",
	}
	cases := []struct {
		name string
		mut  func(o *options)
		want string
	}{
		{"unknown catalog", func(o *options) { o.gauntlet = "other-set" }, "unknown -gauntlet"},
		{"non-FDN set", func(o *options) { o.set = "MH3" }, "-gauntlet requires -set FDN"},
		{"bad baseline", func(o *options) { o.baseline = "coin-flip" }, "unknown -baseline"},
		{"zero games", func(o *options) { o.gauntletGames = 0 }, "-gauntlet-games must be at least 1"},
		{"env policy player", func(o *options) { o.gauntletPlayer = "sb-tactical" }, "bots.Info.Env"},
		{"unknown player", func(o *options) { o.gauntletPlayer = "nosuchpolicy" }, "unknown bot policy"},
	}
	for _, tc := range cases {
		o := base
		tc.mut(&o)
		_, err := run(o, &strings.Builder{})
		if err == nil {
			t.Fatalf("%s: run accepted the gauntlet options, want a refusal", tc.name)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: refusal %q does not name %q", tc.name, err, tc.want)
		}
	}
}
