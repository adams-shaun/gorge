package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/seat"
)

// corpusDir returns the corpus directory or skips: a fresh worktree without
// .cards would otherwise run every corpus test vacuously green. The test
// binary's cwd is this package directory, so the repo root is resolved with
// git (through cards.GitEnv, so an inherited GIT_* does not misdirect it).
func corpusDir(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("git", "-C", ".", "rev-parse", "--show-toplevel")
	cmd.Env = cards.GitEnv()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("could not resolve git repo root: %v", err)
	}
	dir := filepath.Join(strings.TrimSpace(string(out)), ".cards")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("no .cards/ corpus present -- run `make fetch-cards compile-cards`")
	}
	return dir
}

// --- SPSA core -------------------------------------------------------------

// syntheticEvaluator is a deterministic concave objective: the score of a
// profile is minus the squared distance (scaled) from a target vector, and
// the evaluator reports winrate = 0.5 + (score(plus)-score(minus))*gain,
// quantised to an integer win count over `games`. No randomness: the same
// (plus, minus, base) always yields the same result, exactly as the real
// evaluator must.
func syntheticEvaluator(target map[string]int32, gain float64, games int) Evaluator {
	score := func(w Weights) float64 {
		var s float64
		for name, tgt := range target {
			d := float64(getWeight(w, name) - tgt)
			s -= d * d
		}
		return s
	}
	return evalFunc(func(plus, minus Weights, baseSeed uint64) (EvalResult, error) {
		wr := 0.5 + (score(plus)-score(minus))*gain
		if wr < 0 {
			wr = 0
		}
		if wr > 1 {
			wr = 1
		}
		return EvalResult{PlusWins: int(math.Round(wr * float64(games))), Games: games}, nil
	})
}

type evalFunc func(plus, minus Weights, baseSeed uint64) (EvalResult, error)

func (f evalFunc) Eval(plus, minus Weights, baseSeed uint64) (EvalResult, error) {
	return f(plus, minus, baseSeed)
}

// TestSPSAConvergesOnSyntheticConcaveObjective injects a concave evaluator
// (no games played) and asserts the fit moves the tuned weights toward the
// objective's maximum: the final squared distance to the target is smaller
// than the starting one, and at least one weight actually moved.
func TestSPSAConvergesOnSyntheticConcaveObjective(t *testing.T) {
	target := map[string]int32{"CreatureBase": 40, "CreaturePower": 8}
	init := Weights{} // all zero, the worst point for this target
	eval := syntheticEvaluator(target, 0.02, 1000)

	cfg := Config{
		Iters:    300,
		Seed:     42,
		Stride:   8,
		Schedule: SPSASchedule{A: 6.0, Alpha: 0.602, C: 4.0, Gamma: 0.101},
		Fit:      []string{"CreatureBase", "CreaturePower"},
	}
	res, err := RunSPSA(cfg, init, eval, nil)
	if err != nil {
		t.Fatalf("RunSPSA: %v", err)
	}
	if len(res.History) != cfg.Iters {
		t.Fatalf("history = %d iterations, want %d", len(res.History), cfg.Iters)
	}
	dist := func(w Weights) float64 {
		var s float64
		for name, tgt := range target {
			d := float64(getWeight(w, name) - tgt)
			s += d * d
		}
		return s
	}
	if dist(res.Weights) >= dist(init) {
		t.Fatalf("SPSA did not reduce distance to the optimum: start %v (%.0f) final %v (%.0f)",
			init, dist(init), res.Weights, dist(res.Weights))
	}
	if getWeight(res.Weights, "CreatureBase") == 0 && getWeight(res.Weights, "CreaturePower") == 0 {
		t.Fatalf("SPSA never moved a tuned weight: %+v", res.Weights)
	}
	// Frozen weights must stay frozen.
	if res.Weights.NonCreatureCMC != 0 {
		t.Fatalf("an untuned weight moved: NonCreatureCMC = %d", res.Weights.NonCreatureCMC)
	}
}

// TestSPSAPerturbationsAreDistinctAndDeterministic pins the integer-weight
// contract: with c >= 1 the two probes differ on every tuned weight, and the
// same (seed, iteration) always draws the same Rademacher signs.
func TestSPSAPerturbationsAreDistinctAndDeterministic(t *testing.T) {
	d1 := Rademacher(7, 3, 5)
	d2 := Rademacher(7, 3, 5)
	if fmt.Sprint(d1) != fmt.Sprint(d2) {
		t.Fatalf("Rademacher not deterministic: %v vs %v", d1, d2)
	}
	// At c=1, w+cΔ and w-cΔ differ by 2 on every fitted weight.
	for k := 0; k < 5; k++ {
		a, c := (SPSASchedule{A: 1, Alpha: 0.602, C: 1, Gamma: 0.101}).At(k)
		_ = a
		if c < 1 {
			t.Fatalf("iteration %d: c = %v, want >= 1 (integer weights)", k, c)
		}
	}
}

// --- two weight sets reach the two sides -----------------------------------

// TestTwoWeightSetsReachTheTwoSides builds a dev suite whose seat constructor
// records the weights it was handed, runs one real head-to-head evaluation,
// and asserts BOTH the plus and the minus profiles were used -- and that for
// every game seed the two seats received DIFFERENT profiles. This is the
// controller-note requirement: cmd/botbench's package-level cast-profile
// override would put one profile on both sides; the fit must not.
func TestTwoWeightSetsReachTheTwoSides(t *testing.T) {
	dir := corpusDir(t)
	pairs, err := resolvePairs("mono-black-aggro:mono-white-equipment")
	if err != nil {
		t.Fatalf("resolvePairs: %v", err)
	}
	// One pair, one game: exactly two seat constructions, one per side. The
	// per-seat seed derivation (s^1, s^2) means the two calls carry different
	// seeds, so the proof is the weight multiset, not a per-seed grouping.
	suite, err := openDevSuite(dir, pairs, 1, 1, 200, 20000)
	if err != nil {
		t.Fatalf("openDevSuite: %v", err)
	}

	plus := Weights{CreatureBase: 31}
	minus := Weights{CreatureBase: -31}

	var mu sync.Mutex
	seen := map[int32]int{}
	suite.ctor = func(seed uint64, w Weights) seat.Seat {
		mu.Lock()
		seen[w.CreatureBase]++
		mu.Unlock()
		return seat.NewCastProfileBotWithWeights(seed, w)
	}

	if _, err := suite.Eval(plus, minus, 12345); err != nil {
		t.Fatalf("Eval: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if seen[plus.CreatureBase] != 1 || seen[minus.CreatureBase] != 1 {
		t.Fatalf("one game must build each side's profile exactly once, got %v (want %d x1 and %d x1)",
			seen, plus.CreatureBase, minus.CreatureBase)
	}
}

// --- determinism across -workers -------------------------------------------

// TestFitIsDeterministicAcrossWorkers runs the same 3-iteration real smoke
// with -workers 1 and -workers 8 and asserts the trace CSV and the final
// profile are byte-identical: -workers is a scheduling budget only.
func TestFitIsDeterministicAcrossWorkers(t *testing.T) {
	dir := corpusDir(t)
	tmp := t.TempDir()
	run := func(workers int) (trace string, profile string) {
		t.Helper()
		tracePath := filepath.Join(tmp, fmt.Sprintf("trace-%d.csv", workers))
		outPath := filepath.Join(tmp, fmt.Sprintf("profile-%d.json", workers))
		var stdout, stderr bytes.Buffer
		code := mainExit([]string{
			"-dir", dir,
			"-pairs", "mono-black-aggro:mono-white-equipment",
			"-games", "2",
			"-iters", "3",
			"-seed", "5",
			"-workers", fmt.Sprint(workers),
			"-fit", "CreatureBase,CreaturePower,NonCreatureCMC",
			"-trace", tracePath,
			"-out", outPath,
		}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("workers=%d: exit %d, stderr: %s", workers, code, stderr.String())
		}
		tb, err := os.ReadFile(tracePath)
		if err != nil {
			t.Fatalf("workers=%d: reading trace: %v", workers, err)
		}
		pb, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("workers=%d: reading profile: %v", workers, err)
		}
		return string(tb), string(pb)
	}

	trace1, prof1 := run(1)
	trace8, prof8 := run(8)
	if trace1 != trace8 {
		t.Fatalf("trace differs across workers:\n--- workers=1\n%s\n--- workers=8\n%s", trace1, trace8)
	}
	if prof1 != prof8 {
		t.Fatalf("profile differs across workers:\n--- workers=1\n%s\n--- workers=8\n%s", prof1, prof8)
	}
}

// TestRunSmokeCompletes is the brief's smoke: a 3-iteration, 4-game run
// writes /dev/null -- the command completes and the fitted profile loads
// back through L2's loader.
func TestRunSmokeCompletes(t *testing.T) {
	dir := corpusDir(t)
	var stdout, stderr bytes.Buffer
	code := mainExit([]string{
		"-dir", dir,
		"-iters", "3",
		"-games", "4",
		"-workers", "2",
		"-out", "/dev/null",
		"-quiet",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("smoke run exit %d, stderr: %s", code, stderr.String())
	}
}

// TestProfileRoundTripsThroughL2Loader checks the written profile is loadable
// and preserves every field of the fitted weights (the fit's output is the
// L2 input).
func TestProfileRoundTripsThroughL2Loader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	w := Weights{CreatureBase: 33, CreaturePower: -7, CastThreshold: 12, LifeDelta: 3}
	if err := writeProfile(path, w); err != nil {
		t.Fatalf("writeProfile: %v", err)
	}
	got, err := loadInit(path)
	if err != nil {
		t.Fatalf("loadInit: %v", err)
	}
	if got != w {
		t.Fatalf("round-trip = %+v, want %+v", got, w)
	}
}

// --- held-out seed refusal -------------------------------------------------

// TestHeldOutSeedRefusal pins the reserved-range guard at its boundaries: a
// block overlapping [1000000, 2000000) is refused, one ending exactly at the
// boundary is allowed.
func TestHeldOutSeedRefusal(t *testing.T) {
	cases := []struct {
		name string
		base uint64
		span uint64
		want bool
	}{
		{"well below", 1, 4000, false},
		{"ends exactly at lo", heldOutLo - 4000, 4000, false},
		{"spans lo", heldOutLo - 1, 2, true},
		{"inside", 1500000, 10, true},
		{"spans hi", heldOutHi - 1, 2, true},
		{"starts at hi", heldOutHi, 10, false},
		{"zero span", heldOutLo, 0, false},
	}
	for _, tc := range cases {
		if got := seedRangeOverlapsHeldOut(tc.base, tc.span); got != tc.want {
			t.Errorf("%s: seedRangeOverlapsHeldOut(%d, %d) = %v, want %v", tc.name, tc.base, tc.span, got, tc.want)
		}
	}
}

// TestCliRefusesHeldOutSeed pins the refusal at the CLI: a -seed whose whole
// block touches the reserved range exits non-zero with a clear message.
func TestCliRefusesHeldOutSeed(t *testing.T) {
	dir := corpusDir(t)
	var stdout, stderr bytes.Buffer
	// 1 pair x 4 games x 3 iters = 12 seeds from 999995 -> [999995, 1000007).
	code := mainExit([]string{
		"-dir", dir,
		"-pairs", "mono-black-aggro:mono-white-equipment",
		"-games", "4",
		"-iters", "3",
		"-seed", "999995",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("a held-out-overlapping -seed was accepted; stdout: %s", stdout.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("held-out")) {
		t.Fatalf("refusal did not name the held-out range: %s", stderr.String())
	}
}

// TestFitUnknownWeightIsRejected pins the -fit vocabulary check.
func TestFitUnknownWeightIsRejected(t *testing.T) {
	if _, err := resolveFitFields([]string{"NotAWeight"}); err == nil {
		t.Fatal("resolveFitFields accepted an unknown weight name")
	}
}
