package oracle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A pinned game is a recorded game: the two decklists by card name, the seed and the
// canonical actions, kept under testdata/pins and played in lockstep. Nothing about
// it depends on how a synthetic pool deals its pairs, so lowering or refusing a card
// cannot move it; only a change to a card's behaviour, or to which cards lower, can.
//
// Each call plays the games stored for the calling test (a test's n-th call reads
// testdata/pins/<test name>.<n>.json), requires each to reach every condition in
// reached at some decision, and requires the lockstep to stay clean. A pin whose
// conditions are all pinCondition kinds lives in the JSON instead, and
// TestPinManifest plays it (pin_manifest_test.go). A game that no
// longer reaches a condition fails: the recorded actions no longer lead where the
// test is named for, and the games are recorded again with
//
//	PINS_RECORD=1 go test -count=1 -run '^TestName$' ./dirac/oracle
//
// which keeps the recorded games that still reach every condition and play clean, then
// searches the synthetic pool for others that do, up to pinGames. A game recorded with no check ends
// pinTail decisions after its last condition was reached.
const (
	pinGames = 3   // games recorded per call
	pinTail  = 150 // decisions kept after the last condition is reached
)

// pinScan is a var rather than a const only so a test can bound the scan.
var pinScan = 6000

var pinCalls = struct {
	sync.Mutex
	n map[string]int
}{n: map[string]int{}}

// pinPath names the file for the calling test's next pinned-game call.
func pinPath(t *testing.T) string {
	pinCalls.Lock()
	defer pinCalls.Unlock()
	n := pinCalls.n[t.Name()]
	pinCalls.n[t.Name()] = n + 1
	return filepath.Join("testdata", "pins", fmt.Sprintf("%s.%d.json", pinFileStem(t.Name()), n))
}

// pinFileStem turns a test name into a file name a Go module zip accepts: a subtest's
// "/" becomes "__", and anything outside letters, digits, '.', '_' and '-' (a card
// name's apostrophe, say) is dropped, so gorge stays importable as a module.
func pinFileStem(name string) string {
	var b strings.Builder
	for _, r := range strings.ReplaceAll(name, "/", "__") {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// pinnedGames is the recorded form of a synthetic-pool pin: play the games stored for
// this call on the surface cfg names, with each player taking up to one mulligan.
func pinnedGames(t *testing.T, pool string, cfg Config, reached ...func(ref *GorgeSide) bool) {
	t.Helper()
	pinnedGamesRun(t, pool, RunConfig{Config: cfg}, 1, reached...)
}

// pinnedGamesRun is pinnedGames on the run rc with the given mulligan count.
func pinnedGamesRun(t *testing.T, pool string, rc RunConfig, mulligans int, reached ...func(ref *GorgeSide) bool) {
	t.Helper()
	path := pinPath(t)
	reg := synthReg(t)
	set := cardsettest.Synth(t, pool)
	if os.Getenv("PINS_RECORD") == "1" {
		if err := recordPins(t, path, pool, set, rc, mulligans, reached); err != nil {
			t.Fatal(err)
		}
	}
	games, err := readPinGames(path)
	if os.IsNotExist(err) {
		t.Fatalf("no recorded games at %s (%v): record them with PINS_RECORD=1", path, err)
	}
	if err != nil || len(games) == 0 {
		t.Fatalf("%s: %v", path, err)
	}
	playPinnedGames(t, path, pool, set, reg, games, rc, mulligans, reached)
}

// reportPinned fails t unless the game ran clean and reached every condition.
func reportPinned(t *testing.T, what string, res Result, hit []bool) {
	t.Helper()
	if res.Div != nil {
		t.Errorf("%s diverged at %d: %s %s %s %v %v", what, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions)
	}
	if res.Stall != "" && res.Stall != "decisions" {
		t.Errorf("%s stalled: %s", what, res.Stall)
	}
	for i, h := range hit {
		if !h {
			t.Errorf("%s never reaches condition %d of %d: the recorded actions no longer lead there; record the games again (PINS_RECORD=1)", what, i+1, len(hit))
		}
	}
}

// pinStrict reports whether DIRAC_PIN_STRICT=1 holds the pins exact: the
// nightly wants every pin to play and reach its conditions, so a suspended or
// stale pin fails there where a PR run, a batch or the 2-hourly oracle warns
// and passes (the kgen ratchet pins judge the same way).
func pinStrict() bool { return os.Getenv("DIRAC_PIN_STRICT") == "1" }

// pinRefusal is the card and reason of a cardset refusal, when err is one: a
// card the kgen table refuses by name is a declared limit of the corpus, not a
// regression of the engine.
func pinRefusal(err error) (card, reason string, ok bool) {
	var re *cardset.RefusedError
	if !errors.As(err, &re) {
		return "", "", false
	}
	return re.Card, re.Reason, true
}

// stalePin reports whether a recorded game played clean but missed a condition:
// the actions still lead where they lead, just not where the pin is named for.
// A divergence or a stall is never stale: it is the regression the pin exists
// to catch.
func stalePin(res Result, hit []bool) bool {
	return res.Div == nil && (res.Stall == "" || res.Stall == "decisions") && contains(hit, false)
}

// pinVerdict is what playing one recorded game does to its pin.
type pinVerdict int

const (
	pinReport  pinVerdict = iota // ran clean and reached every condition
	pinSuspend                   // the set refuses a card of the recording: the pin cannot run until the card is supported again
	pinStale                     // the actions still play clean but no longer reach a condition
	pinFail                      // a regression, or a suspended or stale pin under DIRAC_PIN_STRICT=1
)

// judgePinned classifies one recorded game's outcome — playPinned's res, hit
// and err — under strict (DIRAC_PIN_STRICT=1, passed in so a test can judge
// both ways without a refused card or a broken recording). A refused card
// suspends the pin, a stale one waits for the caller's fresh-game search, and a
// divergence or a stall fails at once; strict fails a suspended or stale pin
// too, so the nightly still drives the re-records.
func judgePinned(res Result, hit []bool, err error, strict bool) pinVerdict {
	if err != nil {
		if _, _, refused := pinRefusal(err); refused && !strict {
			return pinSuspend
		}
		return pinFail
	}
	if stalePin(res, hit) {
		if strict {
			return pinFail
		}
		return pinStale
	}
	return pinReport
}

// pinStaleGame is a recorded game that played clean but no longer reaches a
// condition, kept so the pin fails as before when no fresh game replaces it.
type pinStaleGame struct {
	what   string
	res    Result
	hit    []bool
	missed int // the first condition not reached, 1-based
}

// playPinnedGames plays each recorded game of the pin at path and reports what
// it found on t. A pinReport game is held to reportPinned, which fails on a
// divergence or a stall; a suspended one passes with a warning outside
// DIRAC_PIN_STRICT=1 and fails under it; a stale one is excused when the pin's
// own pool still holds a game that reaches every condition and plays clean
// (buildPins, bounded by pinScan, writing nothing) and fails as before when it
// does not. It reports whether a stale recording was so excused.
func playPinnedGames(t *testing.T, path, pool string, set cardset.Set, reg *cards.Registry, games []*Repro, rc RunConfig, mulligans int, reached []func(ref *GorgeSide) bool) (replaced bool) {
	t.Helper()
	strict := pinStrict()
	var stale []pinStaleGame
	for i, r := range games {
		what := fmt.Sprintf("%s game %d", path, i)
		res, hit, err := playPinned(set, reg, r, rc, reached)
		switch judgePinned(res, hit, err, strict) {
		case pinReport:
			reportPinned(t, what, res, hit)
		case pinSuspend:
			card, reason, _ := pinRefusal(err)
			t.Logf("WARNING: %s suspended: %s is refused (%s)", what, card, reason)
		case pinStale:
			stale = append(stale, pinStaleGame{what: what, res: res, hit: hit, missed: slices.Index(hit, false) + 1})
		case pinFail:
			if err != nil {
				t.Fatalf("%s: %v", what, err)
			}
			reportPinned(t, what, res, hit)
		}
	}
	if len(stale) == 0 {
		return false
	}
	// The recorded actions still play clean but no longer lead where the pin is
	// named for; the pool may still hold a game that does.
	fresh, err := buildPins(t, path, pool, set, reg, rc, mulligans, reached)
	if err == nil && len(fresh) > 0 {
		for _, g := range stale {
			t.Logf("WARNING: %s is stale (recorded actions no longer reach condition %d of %d); a fresh game reaches it: re-record with PINS_RECORD=1", g.what, g.missed, len(g.hit))
		}
		return true
	}
	for _, g := range stale {
		reportPinned(t, g.what, g.res, g.hit)
	}
	return false
}

// playPinned plays the recorded game r in lockstep, ending with its last action, and
// reports which of the conditions the gorge side reached.
func playPinned(set cardset.Set, reg *cards.Registry, r *Repro, rc RunConfig, reached []func(ref *GorgeSide) bool) (Result, []bool, error) {
	m, err := r.Matchup(reg)
	if err != nil {
		return Result{}, nil, err
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		return Result{}, nil, err
	}
	hit := make([]bool, len(reached))
	inner := rc.Check
	rc.Check = func(ref *GorgeSide, ker Side) *Divergence {
		for i, f := range reached {
			if !hit[i] && f(ref) {
				hit[i] = true
			}
		}
		if inner != nil {
			return inner(ref, ker)
		}
		return nil
	}
	rc.MaxTurns = r.MaxTurns
	rc.MaxDecisions = len(r.Actions) + 1
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		return Result{}, nil, err
	}
	defer CloseSide(k)
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	return res, hit, nil
}

// recordPins writes the games of the synthetic pool that reach every condition and
// play clean to path. It returns an error, and writes nothing, when the scan finds no
// game to record; under PINS_MUTANT=1 that means no game the mutated build diverges in.
func recordPins(t *testing.T, path, pool string, set cardset.Set, rc RunConfig, mulligans int, reached []func(ref *GorgeSide) bool) error {
	t.Helper()
	games, err := buildPins(t, path, pool, set, synthReg(t), rc, mulligans, reached)
	if err != nil {
		return err
	}
	return writePinGames(path, games)
}

// buildPins records the games of the synthetic pool that reach every condition and
// play clean, replacing the games already at path that a change broke.
func buildPins(t *testing.T, path, pool string, set cardset.Set, reg *cards.Registry, rc RunConfig, mulligans int, reached []func(ref *GorgeSide) bool) ([]*Repro, error) {
	t.Helper()
	hit := make([]bool, len(reached))
	checks := 0
	allAt := -1
	inner := rc.Check
	rec := rc
	rec.Check = func(ref *GorgeSide, ker Side) *Divergence {
		checks++
		all := true
		for i, f := range reached {
			if !hit[i] && f(ref) {
				hit[i] = true
			}
			all = all && hit[i]
		}
		if all && allAt < 0 {
			allAt = checks
		}
		if inner != nil {
			return inner(ref, ker)
		}
		return nil
	}
	// PINS_MUTANT=1 records, from a build with the fix under test removed, the games that
	// diverge in it: a condition alone cannot tell a game the fix matters in from one it
	// does not. Record again without it on the restored code to check they run clean.
	mutant := os.Getenv("PINS_MUTANT") == "1"
	play := newSynthPlayerOf(t, pool, rec, mulligans)
	var games []*Repro
	if !mutant {
		games = keptPins(t, path, set, reg, rc, reached)
	}
	kept := len(games)
	for pair := 0; pair < pinScan && len(games) < pinGames; pair++ {
		clear(hit)
		checks, allAt = 0, -1
		m, seed, res, err := play(pair)
		if err != nil { // a pair the set refuses holds no game
			continue
		}
		if mutant {
			// A mutant pin is a game the mutated build diverges in: a clean game
			// recorded here would pass on the mutated tree and guard nothing.
			if res.Div == nil || allAt < 0 {
				continue
			}
			// a game the mutated code diverges in: keep the actions up to the divergence, to
			// be played against the code as it should be
			r := NewRepro(m, seed, rc, &res)
			r.MaxTurns, r.Divergence = 0, nil
			r.Actions = r.Actions[:min(len(r.Actions), res.Div.At+pinTail)]
			// Only write a game the replay actually reaches the divergence in:
			// ScriptPolicy skips actions a decision does not offer, so a recorded
			// action can play out differently.
			got, h, perr := playPinned(set, reg, r, rc, reached)
			if perr != nil || got.Div == nil || contains(h, false) {
				continue
			}
			games = append(games, r)
			continue
		}
		if res.Div != nil || res.Stall != "" || allAt < 0 {
			continue
		}
		if slices.ContainsFunc(games[:kept], func(g *Repro) bool { return g.Seed == seed && g.Decks == m.Names }) {
			continue // a kept game, already in the file
		}
		r := NewRepro(m, seed, rc, &res)
		r.MaxTurns = 0
		full := append([]Canon(nil), r.Actions...)
		if rc.Check == nil {
			r.Actions = r.Actions[:min(len(full), allAt+pinTail)]
		}
		for _, acts := range [][]Canon{r.Actions, full} {
			r.Actions = acts
			got, h, err := playPinned(set, reg, r, rc, reached)
			if err != nil {
				t.Fatal(err)
			}
			if got.Div == nil && (got.Stall == "" || got.Stall == "decisions") && !contains(h, false) {
				games = append(games, r)
				break
			}
		}
	}
	if len(games) == 0 {
		if mutant {
			return nil, fmt.Errorf("PINS_MUTANT=1: no pair of %s within %d diverges after reaching every condition and reproduces on replay", pool, pinScan)
		}
		return nil, fmt.Errorf("no pair of %s within %d reaches every condition and plays clean", pool, pinScan)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Logf("recorded %d games to %s, %d of them kept", len(games), path, kept)
	return games, nil
}

// keptPins is the games already at path that still reach every condition and play clean.
// Recording again replaces only the games a change broke, so an unbroken game keeps its
// decks, seed and actions, and the file's diff names what moved.
func keptPins(t *testing.T, path string, set cardset.Set, reg *cards.Registry, rc RunConfig, reached []func(ref *GorgeSide) bool) []*Repro {
	t.Helper()
	old, err := readPinGames(path)
	if err != nil {
		return nil
	}
	var keep []*Repro
	for _, r := range old {
		got, h, err := playPinned(set, reg, r, rc, reached)
		if err == nil && got.Div == nil && (got.Stall == "" || got.Stall == "decisions") && !contains(h, false) {
			keep = append(keep, r)
		}
	}
	return keep[:min(len(keep), pinGames)]
}

func contains(v []bool, x bool) bool {
	for _, e := range v {
		if e == x {
			return true
		}
	}
	return false
}
