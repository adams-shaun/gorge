package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
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

// ---- deck-pair matrix mode ----

// matrixText runs runPairs + writeMatrixText over the synthetic play and
// returns the text report. It keeps the matrix report tests away from the
// engine (the attribution and CI assertions are testing the report logic,
// not the bot), the same separation the single-pair synthetic tests use.
func matrixText(t *testing.T, games, workers int, pairs []pairDef, play pairPlayer) string {
	t.Helper()
	results, err := runPairs(0, games, "a", "b", pairs, play, workers, nil)
	if err != nil {
		t.Fatalf("runPairs: %v", err)
	}
	var buf bytes.Buffer
	if err := writeMatrixText(&buf, "a", "b", 0, games, results); err != nil {
		t.Fatalf("writeMatrixText: %v", err)
	}
	return buf.String()
}

// TestFullPairsIteratesSorted pins the pair generator: 12 decks must yield
// the 66 unordered pairs, strictly sorted (a < b and lexicographically
// ascending sequence), no duplicates, and the FIRST pair must be the
// historical single-pair run (death-n-taxes : dimir-tempo) so a matrix row
// reproduces it. Sorting is the whole guarantee that pair order never
// depends on map iteration -- the mutation TestMatrixReportOrderIsSorted
// checks the report, this checks the generator.
func TestFullPairsIteratesSorted(t *testing.T) {
	names := testutil.RepoDeckNames()
	if len(names) != 12 {
		t.Fatalf("expected 12 repo decks, got %d: %v", len(names), names)
	}
	ps := fullPairs(names)
	want := len(names) * (len(names) - 1) / 2
	if len(ps) != want {
		t.Fatalf("fullPairs(%d decks) = %d pairs, want %d (= 66)", len(names), len(ps), want)
	}
	if ps[0].String() != "death-n-taxes:dimir-tempo" {
		t.Errorf("first pair = %s, want death-n-taxes:dimir-tempo (the single-pair run must be a matrix row)", ps[0])
	}
	seen := map[string]bool{}
	prev := ""
	for _, p := range ps {
		if p.a >= p.b {
			t.Errorf("pair %s not in a<b order", p)
		}
		s := p.String()
		if seen[s] {
			t.Errorf("duplicate pair %s", s)
		}
		seen[s] = true
		if prev != "" && s <= prev {
			t.Errorf("pairs not strictly ascending after %s: %s (order must be sorted, not map order)", prev, s)
		}
		prev = s
	}
}

// TestParsePairs covers the -pairs spec: "all" expands to every e66pair,
// the named "a:b,c:d" form parses, and an unknown deck name is rejected.
func TestParsePairs(t *testing.T) {
	names := testutil.RepoDeckNames()

	ps, err := parsePairs("all", names)
	if err != nil {
		t.Fatalf("parsePairs(all): %v", err)
	}
	if len(ps) != 66 {
		t.Errorf("parsePairs(all) = %d pairs, want 66", len(ps))
	}

	ps, err = parsePairs("death-n-taxes:dimir-tempo,tron:ur-delver", names)
	if err != nil {
		t.Fatalf("parsePairs(named): %v", err)
	}
	if len(ps) != 2 || ps[0].String() != "death-n-taxes:dimir-tempo" || ps[1].String() != "tron:ur-delver" {
		t.Errorf("parsePairs(named) = %v, want [death-n-taxes:dimir-tempo tron:ur-delver]", ps)
	}

	if _, err := parsePairs("death-n-taxes:not-a-deck", names); err == nil {
		t.Error("parsePairs with an unknown deck name returned no error")
	}
}

// TestMatrixTradesPoliciesEveryGame pins the property that makes a matrix a
// policy measurement rather than a deck measurement: within each pair, seats
// still trade policies every game, so each policy holds each seat in exactly
// half of an even run. If policies stopped trading (e.g. A always on seat 0),
// the deck list a seat holds would masquerade as a policy advantage.
func TestMatrixTradesPoliciesEveryGame(t *testing.T) {
	const games = 6
	byPolicy := map[string]int{} // policy*2+seat -> count
	var mu sync.Mutex
	play := func(pos int, seed uint64, pols []string) (gameOutcome, error) {
		mu.Lock()
		for s := 0; s < 2; s++ {
			byPolicy[pols[s]+"/"+strconv.Itoa(s)]++
		}
		mu.Unlock()
		return gameOutcome{winner: "a", winnerSeat: 0, turns: 5, intents: 10}, nil
	}
	if _, err := runPairs(0, games, "a", "b", []pairDef{{a: "dnt", b: "dim"}}, play, 1, nil); err != nil {
		t.Fatalf("runPairs: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, pol := range []string{"a", "b"} {
		for s := 0; s < 2; s++ {
			want := games / 2
			if got := byPolicy[pol+"/"+strconv.Itoa(s)]; got != want {
				t.Errorf("policy %q held seat %d in %d games, want %d (policies must trade seats within a pair)",
					pol, s, got, want)
			}
		}
	}
}

// TestMatrixGamesArePerPair pins that -games means games PER PAIR, not the
// total: each pair tallies exactly `games` games (A+B+draws == games, not
// the total), the report header states it unambiguously, and the pooled
// total
// is games x the pair count. This is the regression that catches a "make
// -games mean the total" mutation, which a reader of a per-pair row would
// silently misread.
func TestMatrixGamesArePerPair(t *testing.T) {
	const games = 7
	play := func(pos int, seed uint64, pols []string) (gameOutcome, error) {
		return gameOutcome{winner: "a", winnerSeat: 0, turns: 9, intents: 20}, nil
	}
	pairs := []pairDef{{a: "d1", b: "d2"}, {a: "d3", b: "d4"}}
	results, err := runPairs(0, games, "a", "b", pairs, play, 1, nil)
	if err != nil {
		t.Fatalf("runPairs: %v", err)
	}
	for _, r := range results {
		if got := r.aWins + r.bWins + r.draws; got != games {
			t.Errorf("pair %s tallied %d games, want %d per pair (-games is PER PAIR, not total)", r.pd, got, games)
		}
	}
	out := matrixText(t, games, 1, pairs, play)
	if !strings.Contains(out, "games per pair: 7") {
		t.Errorf("report must state games-per-pair explicitly:\n%s", out)
	}
	if !strings.Contains(out, "7 games PER PAIR (total 14 games)") {
		t.Errorf("header must state per-pair count unambiguously:\n%s", out)
	}
}

// TestPooledCIPoolsCountsNotRates pins that the pooled CI is computed over
// pooled COUNTS (sum of wins over sum of games), not by averaging the
// per-pair win rates. With unequal pair sizes the two disagree (0.0 and 0.10
// average to 0.05; 100 in 1010 is 0.099), so a mean-rate pooling mutation is
// caught. Real matrices use equal per-pair N where the two coincide
// numerically -- exactly why this guard needs unequal denominators to prove
// the arithmetic it relies on.
func TestPooledCIPoolsCountsNotRates(t *testing.T) {
	results := []pairResult{
		{pd: pairDef{a: "d1", b: "d2"}, games: 10, aWins: 0, bWins: 10, seatWins: [2]int{0, 10}, totalTurns: 100},
		{pd: pairDef{a: "d3", b: "d4"}, games: 1000, aWins: 100, bWins: 900, seatWins: [2]int{100, 900}, totalTurns: 9000},
	}
	wantRate := 100.0 / 1010.0 // pooled counts, NOT (0.00+0.10)/2 = 0.05
	lo, hi := ci95(100, 1010)
	var buf bytes.Buffer
	// games param only drives the header's per-pair line; the pooled math
	// reads each pair's own game count.
	if err := writeMatrixText(&buf, "a", "b", 0, 10, results); err != nil {
		t.Fatalf("writeMatrixText: %v", err)
	}
	out := buf.String()
	wantPct := wantRate * 100
	wantPctStr := strconv.FormatFloat(wantPct, 'f', 1, 64) // "9.9"
	if !strings.Contains(out, "pooled a win rate: "+wantPctStr+"%") {
		t.Errorf("pooled rate must pool counts (100/1010 = 9.9%%), not average rates (5.0%%):\n%s", out)
	}
	wantLo, wantHi := strconv.FormatFloat(lo*100, 'f', 1, 64), strconv.FormatFloat(hi*100, 'f', 1, 64)
	if !strings.Contains(out, "["+wantLo+"%, "+wantHi+"%]") {
		t.Errorf("pooled CI must be the Wald interval on pooled counts (%s..%s):\n%s", wantLo, wantHi, out)
	}
}

// TestMatrixReportOrderIsSorted pins that the report prints pairs in the
// deterministic sorted generation order (never map iteration) by asserting
// each tabular row's deck pair equals fullPairs' output element for element.
func TestMatrixReportOrderIsSorted(t *testing.T) {
	names := testutil.RepoDeckNames()
	ps := fullPairs(names)
	play := func(pos int, seed uint64, pols []string) (gameOutcome, error) {
		return gameOutcome{winner: "a", winnerSeat: 0, turns: 3, intents: 5}, nil
	}
	results, err := runPairs(0, 1, "a", "b", ps, play, 4, nil)
	if err != nil {
		t.Fatalf("runPairs: %v", err)
	}
	var buf bytes.Buffer
	if err := writeMatrixText(&buf, "a", "b", 0, 1, results); err != nil {
		t.Fatalf("writeMatrixText: %v", err)
	}
	// Table rows are aligned by tabwriter into space padding, so a row's deck
	// pair is its first whitespace field. Match each row against the known
	// pair list (by position) to prove the report follows sorted generation
	// order rather than any map iteration.
	order := make(map[string]int, len(ps))
	for i, p := range ps {
		order[p.String()] = i
	}
	row := 0
	for _, ln := range strings.Split(buf.String(), "\n") {
		fields := strings.Fields(ln)
		if len(fields) == 0 {
			continue
		}
		if idx, ok := order[fields[0]]; ok {
			if idx != row {
				t.Fatalf("pair %q sits at report row %d but its sorted index is %d (report must follow sorted pair order, not map order)",
					fields[0], row, idx)
			}
			row++
		}
	}
	if row != len(ps) {
		t.Errorf("expected %d pair rows in the report, got %d", len(ps), row)
	}
}

// TestMatrixDeterministicUnderWorkers pins that the matrix result and its
// report are byte-identical regardless of how many parallel workers run the
// pairs -- the concurrency the matrix mode adds must not leak into the
// deterministic report. Every pair's tally is written to its own slice
// position and read back in pair order, so worker count is orthogonal to
// the output.
func TestMatrixDeterministicUnderWorkers(t *testing.T) {
	play := func(pos int, seed uint64, pols []string) (gameOutcome, error) {
		if seed%2 == 0 {
			return gameOutcome{winner: "a", winnerSeat: 0, turns: int32(pos), intents: int(seed)}, nil
		}
		return gameOutcome{winner: "b", winnerSeat: 1, turns: int32(seed), intents: int(seed)}, nil
	}
	pairs := fullPairs(testutil.RepoDeckNames())
	r1, err1 := runPairs(3, 50, "a", "b", pairs, play, 1, nil)
	r8, err8 := runPairs(3, 50, "a", "b", pairs, play, 8, nil)
	if err1 != nil || err8 != nil {
		t.Fatalf("runPairs: %v / %v", err1, err8)
	}
	if len(r1) != len(r8) {
		t.Fatalf("result counts differ: %d vs %d", len(r1), len(r8))
	}
	for i := range r1 {
		if r1[i] != r8[i] {
			t.Errorf("result %d differs between workers=1 and workers=8: %+v vs %+v", i, r1[i], r8[i])
		}
	}
	var b1, b2 bytes.Buffer
	if err := writeMatrixText(&b1, "a", "b", 3, 50, r1); err != nil {
		t.Fatal(err)
	}
	if err := writeMatrixText(&b2, "a", "b", 3, 50, r8); err != nil {
		t.Fatal(err)
	}
	if b1.String() != b2.String() {
		t.Errorf("text reports differ between workers=1 and workers=8")
	}
}

// TestMatrixEndToEnd runs the matrix path through the real engine (two
// named pairs, 2 games each) and asserts the report both deterministically
// repeats and contains a row per pair -- the smallest possible matrix is
// still a real two-pair measurement, not a formatting exercise.
func TestMatrixEndToEnd(t *testing.T) {
	dir := corpusDirOrSkip(t)
	pairs, err := parsePairs("death-n-taxes:dimir-tempo,mono-red-goblins:tron", testutil.RepoDeckNames())
	if err != nil {
		t.Fatalf("parsePairs: %v", err)
	}
	var b1, b2 bytes.Buffer
	if err := runMatrix(0, 2, 2, "bot", "bot", dir, "text", pairs, 2, &b1, io.Discard); err != nil {
		t.Fatalf("runMatrix: %v", err)
	}
	out := b1.String()
	for _, want := range []string{"death-n-taxes:dimir-tempo", "mono-red-goblins:tron"} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing pair %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "games per pair: 2") {
		t.Errorf("report must state games-per-pair:\n%s", out)
	}
	// Deterministic: a second identical run is byte-identical.
	if err := runMatrix(0, 2, 2, "bot", "bot", dir, "text", pairs, 2, &b2, io.Discard); err != nil {
		t.Fatalf("runMatrix(second): %v", err)
	}
	if b1.String() != b2.String() {
		t.Errorf("matrix end-to-end report not deterministic across identical runs")
	}
}
