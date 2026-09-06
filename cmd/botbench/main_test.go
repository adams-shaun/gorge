package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// corpusDirOrSkip resolves the repo root the way testutil.CorpusRegistry and
// cmd/mtgsim do -- `git rev-parse --show-toplevel`, never a hard-coded
// relative path -- and Skips when .cards/ is absent, so this package still
// passes on a clean checkout with nothing fetched. The bench drives real
// repo decks through the same engine path a served match uses, so its tests
// need the real corpus; everything else in this package is pure functions
// and needs none.
func corpusDirOrSkip(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "-C", ".", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("could not resolve git repo root: %v", err)
	}
	dir := filepath.Join(strings.TrimSpace(string(out)), ".cards")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("no .cards/ corpus present -- run `make fetch-cards compile-cards`")
	}
	return dir
}

// TestBenchIsDeterministic pins invariant 2: the same (base seed, policies,
// games) twice produces byte-identical output. Nothing in the path --
// engine, seat, seat assignment, report -- may read the wall clock or range
// over a map in an order that reaches the output. Both runs drive the real
// 2-seat engine path (rules.New, per-seat bota, view.Project), so the
// byte-equality covers the whole chain, not just the report formatting.
func TestBenchIsDeterministic(t *testing.T) {
	dir := corpusDirOrSkip(t)
	var b1, b2 bytes.Buffer
	for _, buf := range []*bytes.Buffer{&b1, &b2} {
		if err := run(11, 3, 2, "bot", "bot", dir, buf); err != nil {
			t.Fatalf("run: %v", err)
		}
	}
	if b1.String() != b2.String() {
		t.Fatalf("two runs with identical inputs differed:\n--- run 1 ---\n%s\n--- run 2 ---\n%s",
			b1.String(), b2.String())
	}
}

// TestSeatAssignmentAlternates pins the M1 invariant: over an even N, A
// holds each seat in exactly half the games. With the (game+seat) rule A
// holds seat 0 on even games and seat 1 on odd games, so a seating
// advantage (the first turn is seat 0's, and each seat keeps its own deck
// list for the whole run) is spread equally across the policies instead of
// masquerading as one of them being better.
func TestSeatAssignmentAlternates(t *testing.T) {
	const games, seats = 8, 2
	held := make([]int, seats)
	for g := 0; g < games; g++ {
		for s := 0; s < seats; s++ {
			if aPlaysSeat(g, s) {
				held[s]++
			}
		}
	}
	for s := 0; s < seats; s++ {
		if want := games / 2; held[s] != want {
			t.Errorf("seat %d: A held it %d of %d games, want %d (assignment must alternate)", s, held[s], games, want)
		}
	}
}

// TestTheIntervalWidensOnASmallSample pins the tool's reason for existing: a
// thin sample must report a materially wider interval than a large one at
// the same rate, instead of a misleadingly crisp number. The pure-function
// form (same observed rate .5 at both sample sizes) isolates the effect of
// n -- a test that derived its rates from real game outcomes would be
// measuring the bot, not the interval, and would flake as those outcomes
// change under later bot work. At p̂=.5 the Wald half-width is
// 1.96·√(.25/n) against n, so n=10 is ~4.5x wider than n=200; the
// assertion demands 3x so the regression is unmistakable.
func TestTheIntervalWidensOnASmallSample(t *testing.T) {
	bigLo, bigHi := ci95(100, 200)
	smallLo, smallHi := ci95(5, 10)
	bigW, smallW := bigHi-bigLo, smallHi-smallLo
	if !(smallW > 3*bigW) {
		t.Errorf("interval at n=10 (%.3f..%.3f) must be materially wider than at n=200 (%.3f..%.3f): width %.3f vs %.3f",
			smallLo, smallHi, bigLo, bigHi, smallW, bigW)
	}
}

// TestPerGameSeedVariesWithTheIndex pins M3's non-negotiable: game i must
// be seeded from base+i, so no two games in a run are the same game. Bare
// determinism (TestBenchIsDeterministic) cannot see this mutation -- a
// bench that ground out the same game N times over would still be
// byte-identical run over run -- which is exactly why the per-game line in
// the report prints the seed next to every result: the run shows "the same
// game N times" openly. This test keeps the seed function honest at the
// root instead of waiting for the symptom.
func TestPerGameSeedVariesWithTheIndex(t *testing.T) {
	const base = 7
	for i := 0; i < 16; i++ {
		for j := i + 1; j < 16; j++ {
			si, sj := gameSeed(base, i), gameSeed(base, j)
			if si == sj {
				t.Errorf("gameSeed(%d, %d) == gameSeed(%d, %d) == %d: per-game seeds must vary with the index",
					base, i, base, j, si)
			}
		}
	}
}

// summaryRe is the shape of the report block run() appends after the
// per-game lines; the E2E test below parses it to prove the run actually
// reported, and that the numbers are coherent with each other.
var summaryRe = regexp.MustCompile(
	`(?m)^games played: (\d+)$\n^A wins: (\d+)  B wins: (\d+)  draws: (\d+)$\n` +
		`^A win rate: ([\d.]+)%  95% CI \[([\d.]+)%, ([\d.]+)%\].*$\n^mean turns per game: ([\d.]+)$`)

// TestShortEndToEndRun is the required short end-to-end check: a small N
// actually completes and reports. It is deliberately two games -- the
// whole point of the interval test above is that you do not need many to
// prove the harness works, and every game here is a full 2-seat engine
// play, measured at ~60ms each against the warm corpus cache on this box
// (TEST_HISTORY.md records what the whole suite costs at commit time).
// It asserts the report's arithmetic: the win/draw counts partition the
// games played, the stated rate is the wins over games, the CI brackets
// that rate, and the mean turns is over the same denominator.
func TestShortEndToEndRun(t *testing.T) {
	dir := corpusDirOrSkip(t)
	var buf bytes.Buffer
	if err := run(0, 2, 2, "bot", "bot", dir, &buf); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := buf.String()

	gameLines := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "game ") {
			gameLines++
		}
	}
	if gameLines != 2 {
		t.Errorf("expected 2 per-game lines, got %d:\n%s", gameLines, out)
	}

	m := summaryRe.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("report block missing or malformed:\n%s", out)
	}
	played, _ := strconv.Atoi(m[1])
	aWins, _ := strconv.Atoi(m[2])
	bWins, _ := strconv.Atoi(m[3])
	draws, _ := strconv.Atoi(m[4])
	rate, _ := strconv.ParseFloat(m[5], 64)
	lo, _ := strconv.ParseFloat(m[6], 64)
	hi, _ := strconv.ParseFloat(m[7], 64)
	meanTurns, _ := strconv.ParseFloat(m[8], 64)

	if played != 2 {
		t.Errorf("games played = %d, want 2", played)
	}
	if aWins+bWins+draws != played {
		t.Errorf("A+B+draws = %d+%d+%d = %d, does not partition games played %d",
			aWins, bWins, draws, aWins+bWins+draws, played)
	}
	ratePct := float64(aWins) / float64(played) * 100
	if diff := rate - ratePct; diff > 0.05 || diff < -0.05 {
		t.Errorf("reported rate %.1f%% does not match %d/%d games", rate, aWins, played)
	}
	if lo > rate || hi < rate {
		t.Errorf("CI [%.1f%%, %.1f%%] does not bracket the reported rate %.1f%%", lo, hi, rate)
	}
	if meanTurns <= 0 {
		t.Errorf("mean turns per game = %.1f, want > 0", meanTurns)
	}
}
