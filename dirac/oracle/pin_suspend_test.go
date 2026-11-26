package oracle

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// The stale and suspended pin rules: a recording that plays clean but no longer
// reaches a condition, and a recording the set refuses a card of, are declared
// limits of the corpus, not regressions of the engine. Outside
// DIRAC_PIN_STRICT=1 they pass with a warning; the nightly, which sets it,
// fails them, so a re-record or suspension is still seen daily. A divergence or
// a stall fails either way. judgePinned holds the decision, so these tests pin
// it without a refused card or a broken recording in the corpus.

// TestPinRefusedCardSuspends judges the refusal decision directly: a
// *cardset.RefusedError from the play step suspends the pin outside
// DIRAC_PIN_STRICT=1 and fails under it, and any other play error is a
// regression either way.
func TestPinRefusedCardSuspends(t *testing.T) {
	if pinReport == pinStale || pinSuspend == pinFail {
		t.Fatal("pin verdicts are not distinct: the assertions below cannot fail")
	}
	refused := &cardset.RefusedError{
		Card:   "Iron Man Armor",
		Status: cardset.StatusRefused,
		Reason: "named in dirac/support/refused",
	}
	other := errors.New("seed out of range")
	for _, tc := range []struct {
		name   string
		err    error
		strict bool
		want   pinVerdict
	}{
		{"refused, not strict: suspends", refused, false, pinSuspend},
		{"refused, strict: fails", refused, true, pinFail},
		{"not a refusal, not strict: fails", other, false, pinFail},
		{"not a refusal, strict: fails", other, true, pinFail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := judgePinned(Result{}, nil, tc.err, tc.strict); got != tc.want {
				t.Fatalf("judgePinned(err=%v, strict=%t) = %v, want %v", tc.err, tc.strict, got, tc.want)
			}
		})
	}
}

// TestPinStaleWarnsWhenFreshGameExists judges the stale path on a copy of a
// committed pin whose recorded actions no longer reach the condition: outside
// DIRAC_PIN_STRICT=1 the pin passes with a warning once its pool holds a fresh
// game that reaches every condition, and under strict it fails at once. A
// divergence or a stall is never stale: it fails with strict off too.
func TestPinStaleWarnsWhenFreshGameExists(t *testing.T) {
	// The preconditions the end-to-end assertions depend on: a stale verdict
	// needs a clean play that missed a condition, a divergence and a stall are
	// never excused (reportPinned fails them from the pinReport and pinFail
	// branches), and a clean game that reached everything is only ever reported.
	for _, tc := range []struct {
		name string
		res  Result
		hit  []bool
	}{
		{"diverged, condition missed", Result{Div: &Divergence{At: 3, What: "options"}}, []bool{false}},
		{"diverged, everything reached", Result{Div: &Divergence{At: 3, What: "options"}}, []bool{true}},
		{"stalled at a cap", Result{Stall: "kernel board"}, []bool{true}},
		{"unmodelled stall", Result{Stall: "kernel board", StallUnmodelled: true}, []bool{true}},
	} {
		if got := judgePinned(tc.res, tc.hit, nil, false); got == pinStale || got == pinSuspend {
			t.Fatalf("judgePinned(%s, strict=false) = %v: a regression is never excused", tc.name, got)
		}
	}
	if got := judgePinned(Result{}, []bool{true}, nil, false); got != pinReport {
		t.Fatalf("judgePinned(clean, reached everything, strict=false) = %v, want pinReport", got)
	}

	const src = "testdata/pins/TestAnyTypeRiderSpendsPipsBeforeGeneric.0.json"
	doc, ok, err := readPinDoc(src)
	if err != nil || !ok {
		t.Fatalf("%s: %v (ok=%t)", src, err, ok)
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, doc.Pool)
	rc := RunConfig{Config: doc.Config.config()}
	reached := doc.reached(t)

	// Precondition: the committed games reach every condition as recorded.
	for i, r := range doc.Games {
		_, hit, err := playPinned(set, reg, r, rc, reached)
		if err != nil || contains(hit, false) {
			t.Fatalf("%s game %d: precondition failed: err=%v, hit=%v: the pin must reach every condition before the truncation", src, i, err, hit)
		}
	}

	// Cut the recorded actions back until one game plays clean but no longer
	// reaches every condition: that is the stale recording the rule is for.
	// Reaching a condition only accumulates over a game, so the least prefix
	// that still does is found by a scan back over quarters of the actions.
	staleGame := -1
games:
	for i, r := range doc.Games {
		for _, keep := range []int{3, 2, 1} {
			r2 := *r
			r2.Actions = r.Actions[:max(1, len(r.Actions)*keep/4)]
			res, hit, err := playPinned(set, reg, &r2, rc, reached)
			if err != nil {
				t.Fatalf("%s game %d (first %d of %d actions): %v", src, i, len(r2.Actions), len(r.Actions), err)
			}
			if judgePinned(res, hit, nil, false) == pinStale {
				doc.Games[i] = &r2
				staleGame = i
				break games
			}
		}
	}
	if staleGame < 0 {
		t.Fatal("no quarter of any recorded game misses a condition while playing clean: the test's premise is gone")
	}

	// Under DIRAC_PIN_STRICT=1 the stale recording fails at once.
	t.Setenv("DIRAC_PIN_STRICT", "1")
	res, hit, err := playPinned(set, reg, doc.Games[staleGame], rc, reached)
	if err != nil {
		t.Fatalf("%s game %d (stale): %v", src, staleGame, err)
	}
	if got := judgePinned(res, hit, nil, pinStrict()); got != pinFail {
		t.Fatalf("judgePinned(stale game, DIRAC_PIN_STRICT=1) = %v, want pinFail", got)
	}
	t.Setenv("DIRAC_PIN_STRICT", "")

	// Outside strict the pin passes with a warning once the pool holds a fresh
	// game that reaches every condition and plays clean. The scan is bounded by
	// pinScanTest here, and the file is in a temp dir, so nothing is written.
	defer func(n int) { pinScan = n }(pinScan)
	pinScan = pinScanTest
	path := filepath.Join(t.TempDir(), "TestPinStaleWarnsWhenFreshGameExists.json")
	if err := writePinDoc(path, doc); err != nil {
		t.Fatal(err)
	}
	if replaced := playPinnedGames(t, path, doc.Pool, set, reg, doc.Games, rc, doc.Mulligans, reached); !replaced {
		t.Fatal("the stale recording was not excused: no fresh game of the pool reached every condition and played clean")
	}
}

// pinScanTest bounds the fresh-game search of TestPinStaleWarnsWhenFreshGameExists.
const pinScanTest = 600
