package oracle

import (
	"encoding/json"
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
// reached at some decision, and requires the lockstep to stay clean. A game that no
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
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no recorded games at %s (%v): record them with PINS_RECORD=1", path, err)
	}
	var games []*Repro
	if err := json.Unmarshal(b, &games); err != nil || len(games) == 0 {
		t.Fatalf("%s: %v", path, err)
	}
	for i, r := range games {
		res, hit, err := playPinned(set, reg, r, rc, reached)
		if err != nil {
			t.Fatalf("%s game %d: %v", path, i, err)
		}
		reportPinned(t, fmt.Sprintf("%s game %d", path, i), res, hit)
	}
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
	reg := synthReg(t)
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
			return fmt.Errorf("PINS_MUTANT=1: no pair of %s within %d diverges after reaching every condition and reproduces on replay", pool, pinScan)
		}
		return fmt.Errorf("no pair of %s within %d reaches every condition and plays clean", pool, pinScan)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(games)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("recorded %d games to %s, %d of them kept", len(games), path, kept)
	return nil
}

// keptPins is the games already at path that still reach every condition and play clean.
// Recording again replaces only the games a change broke, so an unbroken game keeps its
// decks, seed and actions, and the file's diff names what moved.
func keptPins(t *testing.T, path string, set cardset.Set, reg *cards.Registry, rc RunConfig, reached []func(ref *GorgeSide) bool) []*Repro {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var old []*Repro
	if json.Unmarshal(b, &old) != nil {
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
