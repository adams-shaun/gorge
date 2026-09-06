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

// summaryRe is the shape of the report block bench() appends after the
// per-game lines; the E2E and attribution tests below parse it to prove the
// run actually reported, that the numbers are coherent with each other, and
// that the seat split is in the output. The A/B line's optional same-policy
// marker suffix is swallowed by the `.*`, and the seat line is the two-seat
// form (a `-seats` run above 2 prints per-seat counts without a rate; no
// test here uses that shape).
//
// Groups: 1 played, 2 A wins, 3 B wins, 4 draws, 5 A rate, 6 CI lo,
// 7 CI hi, 8 seat 0 wins, 9 seat 1 wins, 10 seat-0 rate, 11 seat-0 CI lo,
// 12 seat-0 CI hi, 13 mean turns.
var summaryRe = regexp.MustCompile(
	`(?m)^games played: (\d+)$\n` +
		`^A wins: (\d+)  B wins: (\d+)  draws: (\d+).*$\n` +
		`^A win rate: ([\d.]+)%  95% CI \[([\d.]+)%, ([\d.]+)%\].*$\n` +
		`^seat 0 wins: (\d+)  seat 1 wins: (\d+)  seat 0 win rate: ([\d.]+)%  95% CI \[([\d.]+)%, ([\d.]+)%\].*$\n` +
		`^mean turns per game: ([\d.]+)$`)

// atoi and atof unwrap the summary parse results; the regex already matched,
// so a conversion failure is a bug in the test or the format, not in the
// bench.
func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return n
}

func atof(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(err)
	}
	return f
}

// runSynthetic drives bench with a synthetic match player -- no engine game
// is played -- and returns the parsed summary block. Synthetic outcomes are
// what the attribution tests pin: real game outcomes would couple the
// assertions to what the bot happens to play, which later bot work is
// allowed to change. Both sides are named "bot" in every synthetic run,
// which is exactly the collision the attribution fix is about.
func runSynthetic(t *testing.T, games, seats int, play matchPlayer) []string {
	t.Helper()
	var buf bytes.Buffer
	if err := bench(0, games, seats, "bot", "bot", play, &buf); err != nil {
		t.Fatalf("bench: %v", err)
	}
	m := summaryRe.FindStringSubmatch(buf.String())
	if m == nil {
		t.Fatalf("report block missing or malformed:\n%s", buf.String())
	}
	return m
}

// TestSamePolicyDoesNotCreditOneSideWithEverything is the regression for
// the by-name attribution defect: while both sides were named "bot", every
// non-draw winner matched the -a case first and A was credited with all of
// them -- a confident 100% with a zero-width interval on the only run that
// existed. Attribution must follow the winning seat: A holds seat 0 on even
// games and seat 1 on odd games, so a fixed seat-0 winner over an even N
// splits the run exactly in half. A pass here means the tally reads the
// seat, not the name.
func TestSamePolicyDoesNotCreditOneSideWithEverything(t *testing.T) {
	const games, seats = 8, 2
	m := runSynthetic(t, games, seats, func(_ uint64, _ []string) (gameOutcome, error) {
		return gameOutcome{winner: "bot", winnerSeat: 0, turns: 10, intents: 50}, nil
	})
	aWins, bWins := atoi(m[2]), atoi(m[3])
	if aWins != games/2 || bWins != games/2 {
		t.Errorf("same-policy run, seat 0 won all %d games: A = %d, B = %d, want %d/%d (attribution must follow the seat, not the name)",
			games, aWins, bWins, games/2, games/2)
	}
	// The degenerate line must also say it is degenerate, not read like a
	// policy comparison.
	if !strings.Contains(strings.Join(m, "\n"), "same policy") {
		t.Errorf("same-policy run: A/B line carries no same-policy marker:\n%s", strings.Join(m, "\n"))
	}
}

// TestWinsAreAttributedBySeatNotName pins the direction the tally reads:
// a win from a seat B holds is credited to B even when both sides carry the
// same name. Both games of the run end with seat 1 winning; A holds seat 1
// only on odd games, so exactly one of the two wins is A's -- the other is
// B's. Under the old by-name attribution every winner (named "bot") matched
// the -a case first and A took both.
func TestWinsAreAttributedBySeatNotName(t *testing.T) {
	m := runSynthetic(t, 2, 2, func(_ uint64, _ []string) (gameOutcome, error) {
		return gameOutcome{winner: "bot", winnerSeat: 1, turns: 10, intents: 50}, nil
	})
	aWins, bWins := atoi(m[2]), atoi(m[3])
	if aWins != 1 || bWins != 1 {
		t.Errorf("two seat-1 wins, A held seat 1 once: A = %d, B = %d, want 1/1", aWins, bWins)
	}
	seat0, seat1 := atoi(m[8]), atoi(m[9])
	if seat0 != 0 || seat1 != 2 {
		t.Errorf("seat wins = %d/%d, want 0/2 (both winners sat in seat 1)", seat0, seat1)
	}
}

// TestTheSummaryReportsSeatWins: the seat split is part of the summary, not
// something a reader must reconstruct by grepping the per-game lines. Every
// game's winner sits in seat 1 here, so the seat line must report 0/2 with
// the seat-0 rate at 0% beside its (degenerate, Wald-edge) CI -- the CI
// behaviour itself is TestTheIntervalWidensOnASmallSample's job; this test
// only pins that the line exists in the summary and adds up.
func TestTheSummaryReportsSeatWins(t *testing.T) {
	const games = 2
	m := runSynthetic(t, games, 2, func(_ uint64, _ []string) (gameOutcome, error) {
		return gameOutcome{winner: "bot", winnerSeat: 1, turns: 10, intents: 50}, nil
	})
	seat0, seat1 := atoi(m[8]), atoi(m[9])
	if seat0 != 0 || seat1 != games {
		t.Errorf("seat wins = %d/%d, want 0/%d", seat0, seat1, games)
	}
	if rate := atof(m[10]); rate != 0 {
		t.Errorf("seat 0 win rate = %.1f%%, want 0.0%%", rate)
	}
	// The per-seat counts partition the non-draw wins.
	if seat0+seat1 != atoi(m[2])+atoi(m[3]) {
		t.Errorf("seat wins %d+%d do not match A+B %s+%s", seat0, seat1, m[2], m[3])
	}
}

// TestShortEndToEndRun is the required short end-to-end check: a small N
// actually completes and reports. It is deliberately two games -- the
// whole point of the interval test above is that you do not need many to
// prove the harness works, and every game here is a full 2-seat engine
// play, measured at ~60ms each against the warm corpus cache on this box
// (TEST_HISTORY.md records what the whole suite costs at commit time).
// It asserts the report's arithmetic: the win/draw counts partition the
// games played, the stated rate is the wins over games, the CI brackets
// that rate, the seat split partitions the non-draw wins, and the mean
// turns is over the same denominator.
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

	// With both sides "bot" the A/B line must be marked same-policy, not
	// read like a policy comparison.
	if !strings.Contains(out, "same policy") {
		t.Errorf("same-policy run: A/B line is not marked:\n%s", out)
	}

	m := summaryRe.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("report block missing or malformed:\n%s", out)
	}
	played, aWins, bWins, draws := atoi(m[1]), atoi(m[2]), atoi(m[3]), atoi(m[4])
	rate, lo, hi := atof(m[5]), atof(m[6]), atof(m[7])
	seat0, seat1 := atoi(m[8]), atoi(m[9])
	seatRate := atof(m[10])
	meanTurns := atof(m[13])

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
	if seat0+seat1 != aWins+bWins {
		t.Errorf("seat wins %d+%d do not partition the A+B wins %d+%d", seat0, seat1, aWins, bWins)
	}
	seatRatePct := float64(seat0) / float64(played) * 100
	if diff := seatRate - seatRatePct; diff > 0.05 || diff < -0.05 {
		t.Errorf("seat 0 rate %.1f%% does not match %d/%d games", seatRate, seat0, played)
	}
	if meanTurns <= 0 {
		t.Errorf("mean turns per game = %.1f, want > 0", meanTurns)
	}
}
