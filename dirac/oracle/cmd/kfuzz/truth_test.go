package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
)

// blockSide is a Side that blocks in Step until release closes (a nil channel
// never blocks), and otherwise ends the game at once.
type blockSide struct {
	name    string
	release chan struct{}
}

func (s *blockSide) SideName() string { return s.name }
func (s *blockSide) Step() oracle.CDecision {
	if s.release != nil {
		<-s.release
	}
	return oracle.CDecision{Kind: oracle.DNone}
}
func (s *blockSide) Apply(oracle.Canon)         {}
func (s *blockSide) Terminal() (bool, int8)     { return true, 0 }
func (s *blockSide) Decided() (bool, int8)      { return true, 0 }
func (s *blockSide) Project(*oracle.Projection) {}
func (s *blockSide) Dirty() bool                { return false }
func (s *blockSide) Turn() int                  { return 1 }
func (s *blockSide) Err() error                 { return nil }

// lockstepStub plays one real LockstepLed game between stub sides.
func lockstepStub(release chan struct{}) played {
	res := oracle.LockstepLed(nil, 1, &blockSide{name: "gorge"}, &blockSide{name: "kernel", release: release}, oracle.FirstPolicy(), oracle.RunConfig{})
	return played{res: res}
}

// capture runs f with stdout redirected and returns its code and output.
func capture(t *testing.T, f func() int) (int, string) {
	t.Helper()
	code, stdout, _ := captureStd(t, f)
	return code, stdout
}

func testPairs(n int) []oracle.Pairing {
	ps := make([]oracle.Pairing, n)
	for i := range ps {
		ps[i] = oracle.Pairing{Index: i, A: fmt.Sprintf("deckA%d", i), B: fmt.Sprintf("deckB%d", i), Seed: uint64(100 + i)}
	}
	return ps
}

// reportOf tallies stub outcomes as a run would.
func reportOf(t *testing.T, out []played, absolute bool) (int, string) {
	t.Helper()
	ps := testPairs(len(out))
	o := sweepOpts{lead: "gorge", surface: "autopay", timeout: time.Second, absolute: absolute}
	return capture(t, func() int { return report("fdn", ps, out, oracle.RunConfig{}, o, nil) })
}

// A game that never returns is recorded by name and seed and the worker moves
// on: the one worker here also plays the pair after it.
func TestFuzzHangIsAFailure(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	var calls atomic.Int32
	out := playAll(3, 1, time.Second, true, func(i int) played {
		if i == 1 {
			calls.Add(1)
			return lockstepStub(release)
		}
		return lockstepStub(nil)
	})
	if !out[1].hang || out[0].hang || out[2].hang {
		t.Fatalf("hang flags %v %v %v, want only pair 1", out[0].hang, out[1].hang, out[2].hang)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("pair 1 played %d times, want 2 (one retry, no more)", n)
	}
	for _, absolute := range []bool{false, true} {
		code, stdout := reportOf(t, out, absolute)
		if code != 1 {
			t.Errorf("absolute=%v: exit %d, want 1\n%s", absolute, code, stdout)
		}
		for _, want := range []string{
			"pair 1 (deckA1 vs deckB1, seed 101): hang",
			"fdn: 3 games (0 refused-card, 0 refused-pair, 0 error), 2 clean, 0 divergent, 0 unmodelled [], 0 capacity [], 1 hang (lead gorge, surface autopay)",
			"fdn: 3 games (0 skipped as unsupported), 2 without divergence, 0 kernel capacity stalls [] (lead gorge); first divergences: hang=1",
		} {
			if !strings.Contains(stdout, want) {
				t.Errorf("absolute=%v: no %q in\n%s", absolute, want, stdout)
			}
		}
	}
}

// A game that stalls past the watchdog once (a host stall) and then finishes is
// clean: the watchdog plays it once more before recording a hang.
func TestFuzzHangRetryRecovers(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	var calls atomic.Int32
	out := playAll(3, 1, time.Second, true, func(i int) played {
		if i == 1 && calls.Add(1) == 1 {
			return lockstepStub(release)
		}
		return lockstepStub(nil)
	})
	if n := calls.Load(); n != 2 {
		t.Fatalf("pair 1 played %d times, want 2 (the first attempt timed out)", n)
	}
	if out[0].hang || out[1].hang || out[2].hang {
		t.Fatalf("hang flags %v %v %v, want none", out[0].hang, out[1].hang, out[2].hang)
	}
	code, stdout := reportOf(t, out, true)
	if code != 0 || !strings.Contains(stdout, "3 clean") {
		t.Errorf("exit %d, want 0 with 3 clean\n%s", code, stdout)
	}
}

// A watchdog off (-game-timeout 0) still returns the game.
func TestFuzzNoWatchdogWaits(t *testing.T) {
	out := playAll(2, 2, 0, true, func(int) played { return lockstepStub(nil) })
	if out[0].hang || out[1].hang || !out[0].res.Over || !out[1].res.Over {
		t.Fatalf("outcomes %+v", out)
	}
}

// A panic while building a game fails that pair and leaves the sweep running.
func TestFuzzSetupPanicIsCounted(t *testing.T) {
	out := playAll(3, 1, time.Second, true, func(i int) played {
		if i == 0 {
			panic("table build exploded")
		}
		return lockstepStub(nil)
	})
	code, stdout := reportOf(t, out, false)
	if code != 1 || !strings.Contains(stdout, "2 clean, 1 divergent") || !strings.Contains(stdout, "kfuzz panic=1") {
		t.Errorf("exit %d\n%s", code, stdout)
	}
}

func TestFuzzSkipReasonsCounted(t *testing.T) {
	layer := cardset.LayerRefusal([]string{"A", "B"},
		[]cardset.LayerClasses{{Any: 1, Other: 1}, {Any: 2, Other: 2}},
		[]cardset.LayerConflict{{A: 1, B: 2, Why: "they conflict"}})
	room := cardset.RoomNameRefusal([]string{"Some Room"}, []string{"Some Reader"})
	reduce := cardset.AbReduceRefusal([]string{"Reducer"}, []string{"Filter Land"})
	card := &cardset.RefusedError{Card: "Nope", Status: cardset.StatusRefused, Reason: "refused: lockstep: it differs"}
	for _, e := range []error{layer, room, reduce} {
		if e == nil {
			t.Fatal("a refusal function refused nothing: the fixture is wrong")
		}
		if got := skipClass(e); got != skipRefusedPair {
			t.Errorf("skipClass(%q) = %q, want %s", e, got, skipRefusedPair)
		}
	}
	if got := skipClass(card); got != skipRefusedCard {
		t.Errorf("skipClass(%q) = %q", card, got)
	}
	for _, msg := range []string{`card "X" is not in the pool`, "matchup needs 300 cards", "open deck: no such file", "gorge setup: boom"} {
		if got := skipClass(errors.New(msg)); got != skipError {
			t.Errorf("skipClass(%q) = %q, want %s", msg, got, skipError)
		}
	}
	// The real table's own wording, when a generated pool holds a refused card.
	if set, err := cardset.ByName("blb"); err == nil {
		names, sup := set.Names(), set.Supported()
		for i, ok := range sup {
			if ok || i >= len(names) {
				continue
			}
			_, _, _, err := set.Matchup([]cardset.Entry{{Name: names[i], Count: 1}}, nil)
			if err != nil && strings.Contains(err.Error(), "is not supported") {
				if got := skipClass(err); got != skipRefusedCard {
					t.Errorf("real refusal %q: class %q", err, got)
				}
				break
			}
		}
		_, _, _, err := set.Matchup([]cardset.Entry{{Name: "No Such Card Anywhere", Count: 1}}, nil)
		if err == nil || skipClass(err) != skipError {
			t.Errorf("a card outside the pool: %v", err)
		}
	}

	clean := lockstepStub(nil)
	refused := []played{clean, {skip: layer}, {skip: room}, {skip: reduce}, {skip: reduce}, {skip: card}}
	code, stdout := reportOf(t, refused, true)
	if code != 0 {
		t.Errorf("refusals under -absolute: exit %d, want 0\n%s", code, stdout)
	}
	for _, want := range []string{
		"fdn: 1 games (1 refused-card, 4 refused-pair, 0 error), 1 clean",
		"(5 skipped as unsupported), 1 without divergence",
		"skips: 2 refused-pair: ReduceCost Type Ability beside",
		`skips: 1 refused-card: card "Nope" is not supported`,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("no %q in\n%s", want, stdout)
		}
	}
	withErr := append(refused, played{skip: errors.New("open deck: no such file")})
	code, stdout = reportOf(t, withErr, true)
	if code != 1 || !strings.Contains(stdout, "1 error)") || !strings.Contains(stdout, "pair 6 (deckA6 vs deckB6, seed 106): error: open deck") {
		t.Errorf("error skip under -absolute: exit %d, want 1\n%s", code, stdout)
	}
	if code, stdout = reportOf(t, withErr, false); code != 0 {
		t.Errorf("error skip without -absolute: exit %d, want 0 (counted)\n%s", code, stdout)
	}
}

// TestAbsoluteExitCodes is the exit table in dirac/docs/verification.md.
func TestAbsoluteExitCodes(t *testing.T) {
	clean := lockstepStub(nil)
	unknown := clean
	unknown.res.Stats.Unknown = map[string]int{"priority/manifest_dread": 3}
	for _, c := range []struct {
		name              string
		out               []played
		plain, absoluteRC int
	}{
		{"clean", []played{clean}, 0, 0},
		{"divergence", []played{clean, {res: oracle.Result{Div: &oracle.Divergence{What: "options", Field: "options"}}}}, 1, 1},
		{"hang", []played{clean, {hang: true}}, 1, 1},
		{"kernel panic", []played{clean, {res: oracle.Result{Div: &oracle.Divergence{What: "kernel", Field: "panic", Detail: "index out of range [252]\nstack"}}}}, 1, 1},
		{"capacity stall", []played{clean, {res: oracle.Result{Stall: oracle.KernelStallPrefix + "board"}}}, 0, 0},
		{"turn cap", []played{clean, {res: oracle.Result{Stall: "turns"}}}, 0, 0},
		{"unknown option kind", []played{unknown}, 0, 1},
		{"refused card", []played{clean, {skip: &cardset.RefusedError{Card: "X", Status: cardset.StatusNotLowered, Reason: "not lowered: no"}}}, 0, 0},
		{"refused pair", []played{clean, {skip: cardset.AbReduceRefusal([]string{"R"}, []string{"C"})}}, 0, 0},
		{"error skip", []played{clean, {skip: errors.New("boom")}}, 0, 1},
		{"no game played", nil, 0, 1},
		{"every pair refused", []played{{skip: cardset.AbReduceRefusal([]string{"R"}, []string{"C"})}}, 0, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			if code, stdout := reportOf(t, c.out, false); code != c.plain {
				t.Errorf("plain: exit %d, want %d\n%s", code, c.plain, stdout)
			}
			code, stdout := reportOf(t, c.out, true)
			if code != c.absoluteRC {
				t.Errorf("-absolute: exit %d, want %d\n%s", code, c.absoluteRC, stdout)
			}
			verdictWord := "absolute: OK"
			if c.absoluteRC != 0 {
				verdictWord = "absolute: FAIL"
			}
			if !strings.Contains(stdout, verdictWord) {
				t.Errorf("no %q in\n%s", verdictWord, stdout)
			}
		})
	}
}

// A capacity stall is a declared limit and an unmodelled one a failure, each counted
// under its reason, and the refusals by reason, without card names. The old line
// reads both stalls as it always did.
func TestVerdictSplitsStallClasses(t *testing.T) {
	clean := lockstepStub(nil)
	stall := func(why string, gap bool) played {
		return played{res: oracle.Result{Stall: oracle.KernelStallPrefix + why, StallUnmodelled: gap}}
	}
	refused := played{skip: &cardset.InteractionRefusal{Reason: "infect", Why: "w", Cards: []string{"T", "M"}}}
	for _, c := range []struct {
		name  string
		out   []played
		code  int
		wants []string
	}{
		{"capacity only", []played{clean, stall("board", false), stall("board", false), stall("lki", false)}, 0, []string{
			"fdn: 4 games (0 refused-card, 0 refused-pair, 0 error), 1 clean, 0 divergent, 0 unmodelled [], 3 capacity [board=2 lki=1], 0 hang",
			"3 kernel capacity stalls [board=2 lki=1]",
			"absolute: OK",
		}},
		{"unmodelled", []played{clean, stall("replleave", true), stall("board", false)}, 1, []string{
			"fdn: 3 games (0 refused-card, 0 refused-pair, 0 error), 1 clean, 0 divergent, 1 unmodelled [replleave=1], 1 capacity [board=1], 0 hang",
			"2 kernel capacity stalls [board=1 replleave=1]",
			"pair 1 (deckA1 vs deckB1, seed 101): kernel replleave unmodelled stall",
			"absolute: FAIL: 1 unmodelled",
		}},
		{"a table flag is a refused pair, not a stall", []played{clean, refused, refused, {skip: cardset.AbReduceRefusal([]string{"R"}, []string{"C"})}}, 0, []string{
			"fdn: 1 games (0 refused-card, 3 refused-pair, 0 error), 1 clean, 0 divergent, 0 unmodelled [], 0 capacity [], 0 hang",
			"refused-pair: infect=2 abreduce=1",
			"absolute: OK",
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			code, stdout := reportOf(t, c.out, true)
			if code != c.code {
				t.Errorf("-absolute: exit %d, want %d\n%s", code, c.code, stdout)
			}
			for _, w := range c.wants {
				if !strings.Contains(stdout, w) {
					t.Errorf("no %q in\n%s", w, stdout)
				}
			}
			if code, stdout := reportOf(t, c.out, false); code != 0 {
				t.Errorf("plain: exit %d, want 0: a stall fails only under -absolute\n%s", code, stdout)
			}
		})
	}
}

// Every interaction refusal is a refused pair, whatever its message says.
func TestSkipClassReadsTheInteractionRefusalType(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", &cardset.InteractionRefusal{Reason: "infect", Why: "w", Cards: []string{"A", "B"}})
	if got := skipClass(err); got != skipRefusedPair {
		t.Errorf("skipClass = %q, want %s", got, skipRefusedPair)
	}
	if got := pairReason(err); got != "infect" {
		t.Errorf("pairReason = %q", got)
	}
}

// The vanilla path's watchdog: a game that never returns is named, the run moves on
// and fails, and a clean run still prints the line its parsers read.
func TestFuzzVanillaHangIsAFailure(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	code, stdout := capture(t, func() int {
		return vanillaSweep(0, 3, 4, 2, 7, time.Second, oracle.RunConfig{}, "", "gorge", nil, func(g int) played {
			if g == 1 {
				return lockstepStub(release)
			}
			return lockstepStub(nil)
		})
	})
	if code != 1 {
		t.Errorf("exit %d, want 1\n%s", code, stdout)
	}
	for _, want := range []string{"game 1 (vanilla 1 vs", "hang: no result after two watchdogs of 1s", "vanilla: 3 games, 2 without divergence (lead gorge); first divergences: hang=1"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("no %q in\n%s", want, stdout)
		}
	}
	code, stdout = capture(t, func() int {
		return vanillaSweep(0, 2, 4, 2, 7, time.Second, oracle.RunConfig{}, "", "gorge", nil, func(int) played { return lockstepStub(nil) })
	})
	if code != 0 || !strings.Contains(stdout, "vanilla: 2 games, 2 without divergence") {
		t.Errorf("clean: exit %d\n%s", code, stdout)
	}
}

// The generic path's watchdog (oracle.Fuzz): a side that never answers is a hang,
// counted and named, the other pairs still play, and it can be switched off.
func TestFuzzGenericHangIsAFailure(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	decks := oracle.PoolDecks("fdn")
	if err := oracle.NeedDecks(decks); err != nil {
		t.Skipf("FDN pool absent: %v", err)
	}
	c, err := oracle.OpenCorpus(corpus, decks)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := oracle.ReadPool(filepath.Join(filepath.Dir(decks), "pool.tsv"), "all")
	if err != nil {
		t.Fatal(err)
	}
	ps := oracle.PairingsFrom(pool, 0, 3, 1)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	inner := oracle.GorgeKernel(oracle.Config{})
	mk := func(m *oracle.Matchup, seed uint64) (oracle.Side, error) {
		if seed == ps[1].Seed {
			return &blockSide{name: "kernel", release: release}, nil
		}
		return inner(m, seed)
	}
	sum := oracle.Fuzz(c, ps, mk, oracle.FuzzConfig{Run: oracle.RunConfig{MaxDecisions: 30}, GameTimeout: 2 * time.Second, Workers: 2}, nil)
	if sum.Hung != 1 || len(sum.Hangs) != 1 || sum.Hangs[0].Index != ps[1].Index || sum.ByClass["hang"] != 1 {
		t.Errorf("hung %d %+v classes %v, want pair %d", sum.Hung, sum.Hangs, sum.ByClass, ps[1].Index)
	}
	if sum.Games != 3 {
		t.Errorf("%d games, want the hang and the two others", sum.Games)
	}
}

// -mulligans reaches a pool run: a game plays the round when asked and skips it at
// the default, which is the run the flag's absence always meant.
func TestFuzzPoolRunPlaysMulligans(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	decks := oracle.PoolDecks("fdn")
	if err := oracle.NeedDecks(decks); err != nil {
		t.Skipf("FDN pool absent: %v", err)
	}
	c, err := oracle.OpenCorpus(corpus, decks)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := oracle.ReadPool(filepath.Join(filepath.Dir(decks), "pool.tsv"), "all")
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.ByName("fdn")
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.AllowUnverified(set) // the first pairs' decks, whatever their evidence
	ps := oracle.PairingsFrom(pool, 0, 4, 1)
	rc := oracle.RunConfig{MaxDecisions: 2000, MaxTurns: 100}
	// A pair holding a card refused by name is skipped and asks nothing.
	asked := func(mulligans int) (keepOrMull, games int) {
		code, _ := capture(t, func() int {
			return runFDN(c, set, rc, ps, sweepOpts{lead: "gorge", surface: "full", workers: 1, mulligans: mulligans, onGame: func(res *oracle.Result) {
				games++
				for _, a := range res.Actions {
					if k := a.Kind(); k == oracle.ActKeep || k == oracle.ActMulligan {
						keepOrMull++
					}
				}
			}}, oracle.RandomPolicy)
		})
		if code != 0 {
			t.Errorf("-mulligans %d: exit %d", mulligans, code)
		}
		return keepOrMull, games
	}
	if n, _ := asked(0); n != 0 {
		t.Errorf("-mulligans 0 asked %d keep-or-mulligan decisions, want none", n)
	}
	if n, games := asked(1); games == 0 || n < 2*games {
		t.Errorf("-mulligans 1 asked %d keep-or-mulligan decisions over %d games, want at least two a game", n, games)
	}
}

// The forms a script reads the verdict by. One takes the first line starting with
// the pool's name.
func TestFuzzVerdictKeepsParsersWorking(t *testing.T) {
	out := []played{lockstepStub(nil), lockstepStub(nil), {res: oracle.Result{Stall: oracle.KernelStallPrefix + "board"}},
		{res: oracle.Result{Div: &oracle.Divergence{What: "options", Field: "options"}}}, {hang: true}, {skip: errors.New("boom")}}
	_, stdout := reportOf(t, out, false)
	var fdn []string
	for _, l := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(l, "fdn:") {
			fdn = append(fdn, l)
		}
	}
	if len(fdn) != 2 {
		t.Fatalf("want the legacy and new verdict lines, got %q", fdn)
	}
	gate := regexp.MustCompile(`.* ([0-9]+) without divergence.*`) // the clean count
	if m := gate.FindStringSubmatch(strings.Join(fdn, "\n")); m == nil || m[1] != "2" {
		t.Errorf("the clean count: %v over %q", m, fdn)
	}
	pool := regexp.MustCompile(`.*, ([0-9][0-9]*) without divergence.*`) // on the first line
	if m := pool.FindStringSubmatch(fdn[0]); m == nil || m[1] != "2" {
		t.Errorf("the first line: %v over %q", m, fdn[0])
	}
	// the summary form; diverged = games - clean - stalls
	summary := regexp.MustCompile(`(?m)^fdn: (\d+) games \((\d+) skipped as unsupported\), (\d+) without divergence, (\d+) kernel capacity stalls(?: \[[^\]]*\])? \(lead (\w+)\); first divergences: (.*)$`)
	m := summary.FindStringSubmatch(stdout)
	if m == nil {
		t.Fatalf("the summary form matches nothing in\n%s", stdout)
	}
	if m[1] != "5" || m[2] != "1" || m[3] != "2" || m[4] != "1" || m[5] != "gorge" {
		t.Errorf("SUMMARY groups %q", m[1:])
	}
}

// Only clean games reach -coverage and -evidence. The two cards that appear only
// in a dirty game must not.
func TestFuzzCoverageMergesCleanGamesOnly(t *testing.T) {
	dir := t.TempDir()
	coveragePath, evidencePath = filepath.Join(dir, "cov.tsv"), filepath.Join(dir, "evidence.txt")
	t.Cleanup(func() { coveragePath, evidencePath = "", "" })
	game := func(card string) *oracle.Coverage {
		c := oracle.NewCoverage()
		c.Cards[card] = &oracle.CardCoverage{Cast: oracle.EvidenceMinPlays, Used: map[string]int{}}
		c.Games = 1
		return c
	}
	clean := lockstepStub(nil)
	clean.cov = game("Clean Card")
	dirty := played{res: oracle.Result{Div: &oracle.Divergence{What: "options", Field: "options"}}, cov: game("Diverged Card")}
	gap := played{res: oracle.Result{Stall: oracle.KernelStallPrefix + "replleave", StallUnmodelled: true}, cov: game("Unmodelled Card")}
	limit := played{res: oracle.Result{Stall: oracle.KernelStallPrefix + "board"}, cov: game("Capacity Card")}
	code, stdout := reportOf(t, []played{clean, dirty, gap, limit, {hang: true}}, false)
	if code != 1 {
		t.Errorf("exit %d, want 1 for the divergence\n%s", code, stdout)
	}
	tsv, err := os.ReadFile(coveragePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Clean Card\t", "Capacity Card\t"} {
		if !bytes.Contains(tsv, []byte(want)) {
			t.Errorf("%s is missing from\n%s", want, tsv)
		}
	}
	for _, bad := range []string{"Diverged Card", "Unmodelled Card"} {
		if bytes.Contains(tsv, []byte(bad)) {
			t.Errorf("%s reached the coverage:\n%s", bad, tsv)
		}
	}
	if !strings.Contains(stdout, "in 2 games)") {
		t.Errorf("coverage summary should count 2 games:\n%s", stdout)
	}
	ev, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(ev) != "Capacity Card\nClean Card\n" {
		t.Errorf("evidence %q, want the two clean games' cards", ev)
	}
}

func TestFuzzVerdictRanksImplicated(t *testing.T) {
	ms := &oracle.Matchup{Table: []string{"Alpha", "Rockface Village"}, Deck: [2][]uint8{{0}, {1}}}
	div := func(opt string) played {
		return played{m: ms, res: oracle.Result{Div: &oracle.Divergence{What: "options", Field: "options", RefOptions: []string{opt}}}}
	}
	code, stdout := reportOf(t, []played{div("activate(#1,-,2)"), div("activate(#1,-,2)"), div("cast(#0,-,0)")}, true)
	if code != 1 || !strings.Contains(stdout, "implicates: Rockface Village=2, Alpha=1\n") {
		t.Errorf("exit %d\n%s", code, stdout)
	}
	var many []played
	for i := 0; i < 12; i++ {
		tbl := []string{fmt.Sprintf("Card %02d", i)}
		many = append(many, played{m: &oracle.Matchup{Table: tbl, Deck: [2][]uint8{{0}}}, res: oracle.Result{Div: &oracle.Divergence{What: "options", RefOptions: []string{"cast(#0,-,0)"}}}})
	}
	_, stdout = reportOf(t, many, true)
	if !strings.Contains(stdout, "Card 09=1\n") || strings.Contains(stdout, "Card 10") {
		t.Errorf("want the top 10:\n%s", stdout)
	}
	// -v prints it under each divergence.
	verboseFDN = true
	t.Cleanup(func() { verboseFDN = false })
	_, stdout = reportOf(t, []played{div("activate(#1,-,2)")}, true)
	if !strings.Contains(stdout, "  implicates: Rockface Village\n") {
		t.Errorf("-v: no implicates line under the divergence:\n%s", stdout)
	}
}

func TestFuzzSurfaceFlag(t *testing.T) {
	for _, c := range []struct {
		name     string
		alias    bool
		aliasSet bool
		want     string
		wantErr  bool
	}{
		{"", false, false, "full", false},
		{"", true, true, "autopay", false},
		{"", false, true, "full", false},
		{"autopay", false, false, "autopay", false},
		{"full", false, false, "full", false},
		{"autopay", true, true, "autopay", false},
		{"full", false, true, "full", false},
		{"full", true, true, "", true},
		{"autopay", false, true, "", true},
		{"bogus", false, false, "", true},
	} {
		got, err := resolveSurface(c.name, c.alias, c.aliasSet)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("resolveSurface(%q, %v, %v) = %q, %v; want %q, err %v", c.name, c.alias, c.aliasSet, got, err, c.want, c.wantErr)
		}
	}
}

func TestPairsFileReproducesPairing(t *testing.T) {
	var pool []oracle.PoolDeck
	for i := 0; i < 12; i++ {
		pool = append(pool, oracle.PoolDeck{Name: fmt.Sprintf("deck%02d", i)})
	}
	const seed = 7
	want := oracle.PairingsFrom(pool, 0, 6, seed)
	var b strings.Builder
	b.WriteString("# slice\n\n")
	for _, p := range want {
		fmt.Fprintf(&b, "%s\t%s\t%d\n", p.A, p.B, p.Seed)
	}
	path := filepath.Join(t.TempDir(), "pairs.tsv")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	same := func(got, want []oracle.Pairing) {
		t.Helper()
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("got %v\nwant %v", got, want)
		}
	}
	got, err := pairsFromFile(path, 0, 100, false, 999) // a written seed ignores -seed; no -pairs means every line
	if err != nil {
		t.Fatal(err)
	}
	same(got, want)
	got, err = pairsFromFile(path, 2, 2, true, seed)
	if err != nil {
		t.Fatal(err)
	}
	same(got, want[2:4]) // -first-pair K -pairs 1 re-runs the pair reported as K
	if got, err = pairsFromFile(path, 40, 2, true, seed); err != nil || len(got) != 0 {
		t.Errorf("past the end: %v %v", got, err)
	}

	// No seed on the line: oracle.PairSeed's rule.
	noSeed := filepath.Join(t.TempDir(), "pairs.tsv")
	if err := os.WriteFile(noSeed, []byte("a\tb\nc\td\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = pairsFromFile(noSeed, 0, 0, false, seed)
	if err != nil {
		t.Fatal(err)
	}
	s1, _ := oracle.PairSeed(oracle.PairSpec{A: "c", B: "d"}, 1, seed)
	if len(got) != 2 || got[1].Seed != s1 || got[1].Index != 1 || got[0].Seed == got[1].Seed {
		t.Errorf("derived seeds: %v, want pair 1's %d", got, s1)
	}
	bad := filepath.Join(t.TempDir(), "pairs.tsv")
	if err := os.WriteFile(bad, []byte("a\tb\tnotaseed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := pairsFromFile(bad, 0, 0, false, seed); err == nil {
		t.Error("a bad seed: want an error")
	}
	if _, err := pairsFromFile(filepath.Join(t.TempDir(), "missing"), 0, 0, false, seed); err == nil {
		t.Error("a missing file: want an error")
	}
}

// verdictLines keeps a run's fdn: lines.
func verdictLines(stdout string) string {
	var b strings.Builder
	for _, l := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(l, "fdn:") || strings.HasPrefix(l, "fdn-classes-json:") {
			b.WriteString(l + "\n")
		}
	}
	return b.String()
}

// TestFuzzPoolRunFlags drives the new flags through run() on real games.
func TestFuzzPoolRunFlags(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	fdnDecks := oracle.PoolDecks("fdn")
	if _, err := os.Stat(filepath.Join(filepath.Dir(fdnDecks), "pool.tsv")); err != nil {
		t.Skipf("FDN pool absent: %v", err)
	}
	if _, err := os.Stat(fdnDecks); err != nil {
		t.Skipf("FDN decks absent: %v", err)
	}
	// Pairs 8 to 10 hold no card refused by name, so all three play.
	slice := []string{"-kernel", "fdn", "-first-pair", "8", "-pairs", "3", "-seed", "1", "-keep-going", "-shrink", "0", "-allow-unverified"}
	withFlags := func(extra ...string) []string { return append(append([]string{}, slice...), extra...) }

	code, canon, stderr := runKFuzz(t, withFlags("-surface", "autopay")...)
	if code != 0 || !strings.Contains(canon, "3 clean") || !strings.Contains(canon, "surface autopay)") {
		t.Fatalf("-surface autopay: exit %d\n%s\n%s", code, canon, stderr)
	}
	// The old spelling plays the same run.
	code, alias, stderr := runKFuzz(t, withFlags("-autopay-surface")...)
	if code != 0 || verdictLines(alias) != verdictLines(canon) {
		t.Errorf("-autopay-surface: exit %d\n%s\nwant\n%s\n%s", code, alias, canon, stderr)
	}
	if code, stdout, _ := runKFuzz(t, withFlags("-surface", "full")...); !strings.Contains(stdout, "surface full)") {
		t.Errorf("-surface full: exit %d\n%s", code, stdout)
	}
	if code, _, _ := runKFuzz(t, withFlags("-surface", "full", "-autopay-surface")...); code != 2 {
		t.Errorf("contradicting surfaces: exit %d, want 2", code)
	}
	if code, _, _ := runKFuzz(t, withFlags("-surface", "bogus")...); code != 2 {
		t.Errorf("-surface bogus: exit %d, want 2", code)
	}

	// -absolute reaches the exit code.
	if code, stdout, _ := runKFuzz(t, withFlags("-surface", "autopay", "-absolute")...); code != 0 || !strings.Contains(stdout, "absolute: OK") {
		t.Errorf("-absolute clean: exit %d\n%s", code, stdout)
	}
	mutateKernel(true)
	code, stdout, _ := runKFuzz(t, "-kernel", "fdn", "-first-pair", strconv.Itoa(mutatedPair), "-pairs", "1", "-seed", "1", "-surface", "autopay", "-shrink", "0", "-absolute")
	mutateKernel(false)
	if code != 1 || !strings.Contains(stdout, "absolute: FAIL: 1 divergent") {
		t.Errorf("-absolute divergent: exit %d\n%s", code, stdout)
	}
	for _, k := range []string{"vanilla", "gorge"} {
		if code, _, stderr := runKFuzz(t, "-kernel", k, "-absolute"); code != 2 || !strings.Contains(stderr, "-absolute needs") {
			t.Errorf("-kernel %s -absolute: exit %d, stderr %q", k, code, stderr)
		}
	}

	// A real game that outlives its time is a hang, named, and no pair is lost.
	code, stdout, _ = runKFuzz(t, withFlags("-surface", "autopay", "-game-timeout", "1ns")...)
	if code != 1 || !strings.Contains(stdout, "3 hang") || !regexp.MustCompile(`pair 8 \(\S+ vs \S+, seed \d+\): hang`).MatchString(stdout) {
		t.Errorf("-game-timeout 1ns: exit %d\n%s", code, stdout)
	}

	// -pairs-file plays what the pool draws.
	pool, err := oracle.ReadPool(filepath.Join(filepath.Dir(fdnDecks), "pool.tsv"), "all")
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, p := range oracle.PairingsFrom(pool, 8, 3, 1) {
		fmt.Fprintf(&b, "%s\t%s\t%d\n", p.A, p.B, p.Seed)
	}
	file := filepath.Join(t.TempDir(), "pairs.tsv")
	if err := os.WriteFile(file, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	code, fromFile, _ := runKFuzz(t, "-kernel", "fdn", "-pairs-file", file, "-keep-going", "-surface", "autopay", "-shrink", "0", "-allow-unverified")
	if code != 0 || verdictLines(fromFile) != verdictLines(canon) {
		t.Errorf("-pairs-file: exit %d\n%s\nwant\n%s", code, fromFile, canon)
	}

	// -evidence holds names the coverage shows at the floor.
	dir := t.TempDir()
	ev, covPath := filepath.Join(dir, "ev.txt"), filepath.Join(dir, "cov.tsv")
	code, stdout, _ = runKFuzz(t, "-kernel", "fdn", "-first-pair", "0", "-pairs", "20", "-seed", "1", "-surface", "autopay", "-workers", "2", "-coverage", covPath, "-evidence", ev)
	if code != 0 || !strings.Contains(stdout, "evidence: ") {
		t.Fatalf("-evidence: exit %d\n%s", code, stdout)
	}
	raw, err := os.ReadFile(ev)
	if err != nil {
		t.Fatal(err)
	}
	covRaw, err := os.ReadFile(covPath)
	if err != nil {
		t.Fatal(err)
	}
	names := strings.Fields(strings.ReplaceAll(strings.TrimSpace(string(raw)), " ", "_"))
	if len(names) == 0 {
		t.Fatalf("no card reached the floor in 20 games:\n%s", stdout)
	}
	for _, n := range names {
		if !bytes.Contains(covRaw, []byte(strings.ReplaceAll(n, "_", " ")+"\t")) {
			t.Errorf("evidence names %q, absent from the coverage", n)
		}
	}
}

// The generic oracle.Fuzz path retries a timed-out game too: pair 1's kernel blocks
// only on its first build, so the retry finishes and nothing hangs.
func TestFuzzGenericHangRetryRecovers(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	decks := oracle.PoolDecks("fdn")
	if err := oracle.NeedDecks(decks); err != nil {
		t.Skipf("FDN pool absent: %v", err)
	}
	c, err := oracle.OpenCorpus(corpus, decks)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := oracle.ReadPool(filepath.Join(filepath.Dir(decks), "pool.tsv"), "all")
	if err != nil {
		t.Fatal(err)
	}
	ps := oracle.PairingsFrom(pool, 0, 3, 1)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	inner := oracle.GorgeKernel(oracle.Config{})
	var seen atomic.Int32
	mk := func(m *oracle.Matchup, seed uint64) (oracle.Side, error) {
		if seed == ps[1].Seed && seen.Add(1) == 1 {
			return &blockSide{name: "kernel", release: release}, nil
		}
		return inner(m, seed)
	}
	sum := oracle.Fuzz(c, ps, mk, oracle.FuzzConfig{Run: oracle.RunConfig{MaxDecisions: 30}, GameTimeout: 2 * time.Second, Workers: 2}, nil)
	if n := seen.Load(); n != 2 {
		t.Fatalf("pair %d built %d times, want 2 (the first attempt timed out)", ps[1].Index, n)
	}
	if sum.Hung != 0 || len(sum.Hangs) != 0 {
		t.Errorf("hung %d %+v, want none", sum.Hung, sum.Hangs)
	}
	if sum.Games != 3 {
		t.Errorf("%d games, want 3", sum.Games)
	}
}
