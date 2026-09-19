// SPSA core for cmd/policytune: simultaneous-perturbation stochastic
// approximation over a cast-weight profile, with common random numbers.
//
// The fitter is deliberately small and injection-friendly: the objective
// arrives as an Evaluator (the real one plays engine games on the dev suite;
// a test injects a synthetic concave function), so the optimization loop can
// be proven to converge without paying for a single game. Nothing here reads
// the wall clock, ranges over a map whose order can reach the output, or uses
// the global math/rand -- a fit is a pure function of its flags.
package main

import (
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"reflect"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/botpolicy"
)

// Weights is the cast-weight profile the fitter moves.
type Weights = botpolicy.CastWeights

// EvalResult is one evaluation's outcome: how often the "+" candidate beat
// the "-" candidate over the dev suite, and over how many games that rate is
// taken (stalled games are excluded, exactly as the bench excludes them).
type EvalResult struct {
	PlusWins int
	Games    int
}

// WinRate is the "+" side's win rate over non-stalled games. A suite whose
// every game stalled has no rate at all; 0.5 (break-even) is returned so the
// gradient contributes nothing rather than a spurious extreme.
func (r EvalResult) WinRate() float64 {
	if r.Games <= 0 {
		return 0.5
	}
	return float64(r.PlusWins) / float64(r.Games)
}

// Evaluator plays one head-to-head evaluation. baseSeed is the common-random-
// numbers block the evaluation must use for ALL of its games, so the "+" and
// "-" profiles see identical per-game seeds (the variance reduction SPSA
// depends on). Implementations must be deterministic: the same
// (plus, minus, baseSeed) always yields the same result.
type Evaluator interface {
	Eval(plus, minus Weights, baseSeed uint64) (EvalResult, error)
}

// SPSASchedule is the gain schedule. a_k = A/(k+1)^Alpha shrinks the step;
// c_k = max(1, round(C/(k+1)^Gamma)) shrinks the perturbation but never below
// 1, because the weights are integers: a c_k below 1 would round both
// w+cΔ and w-cΔ back to w and the iteration would probe nothing.
type SPSASchedule struct {
	A, Alpha float64
	C, Gamma float64
}

// DefaultSchedule is the textbook finite-difference SPSA schedule with the
// "practical" exponents (Spall 1998): a decays 0.602, c decays 0.101.
var DefaultSchedule = SPSASchedule{A: 1.0, Alpha: 0.602, C: 4.0, Gamma: 0.101}

// Schedule returns the (a, c) pair for iteration k (0-based).
func (s SPSASchedule) At(k int) (a, c float64) {
	denomA := math.Pow(float64(k+1), s.Alpha)
	denomC := math.Pow(float64(k+1), s.Gamma)
	a = s.A / denomA
	c = math.Round(s.C / denomC)
	if c < 1 {
		c = 1
	}
	return a, c
}

// Rademacher returns the perturbation signs for one iteration: one ±1 per
// fitted weight, drawn from a PRNG seeded by (seed, iter). Deriving the
// stream from (seed, iter) rather than carrying one PRNG across iterations
// makes iteration k's perturbation a pure function of the two, so a fit is
// reproducible even if only some iterations are re-run.
func Rademacher(seed uint64, iter, n int) []int {
	r := rand.New(rand.NewPCG(seed, uint64(iter)))
	out := make([]int, n)
	for i := range out {
		if r.Uint64()&1 == 0 {
			out[i] = 1
		} else {
			out[i] = -1
		}
	}
	return out
}

// TraceWriter is the optional per-iteration CSV trajectory.
type TraceWriter interface {
	WriteRow(iter int, a, c float64, res EvalResult, w Weights) error
	Close() error
}

// BenchFn, when non-nil, benches the current weights against the production
// bot and returns the win rate. RunSPSA calls it every BenchEvery iterations.
type BenchFn func(iter int, w Weights) (float64, error)

// Config configures one fit. Fit names the CastWeights fields to tune; the
// rest stay frozen at their init values (a zero-length Fit means every field).
type Config struct {
	Iters      int
	Seed       uint64
	Stride     uint64 // seeds consumed per evaluation (pairs * games)
	Schedule   SPSASchedule
	Fit        []string
	BenchEvery int
	Trace      TraceWriter
	Log        io.Writer
}

// Result is a completed fit's outcome.
type Result struct {
	Weights Weights
	History []Iteration
}

// Iteration is one recorded SPSA step, for the trace and for tests.
type Iteration struct {
	Index  int
	A, C   float64
	Result EvalResult
	Plus   Weights
	Minus  Weights
	Next   Weights
}

// RunSPSA runs the fit: iteration k draws Rademacher Δ from (seed, k), builds
// w+cΔ and w-cΔ as INTEGER profiles (rounding each perturbed weight, c>=1 so
// the two differ), evaluates them head-to-head on the same per-game seeds
// (baseSeed = seed + k*Stride), and steps
//
//	ĝ_i = (winrate - 0.5) * 2 / (2 c Δ_i)
//	w_i ← round(w_i + a ĝ_i)
//
// Every BenchEvery iterations (when > 0) it also benches the current weights
// against the bot and logs the rate. The returned weights are the final
// profile; History carries every step.
func RunSPSA(cfg Config, init Weights, eval Evaluator, bench BenchFn) (Result, error) {
	if cfg.Iters < 1 {
		return Result{Weights: init}, fmt.Errorf("-iters must be at least 1, got %d", cfg.Iters)
	}
	fields, err := resolveFitFields(cfg.Fit)
	if err != nil {
		return Result{}, err
	}
	if len(fields) == 0 {
		return Result{}, fmt.Errorf("-fit names no weights")
	}

	w := init
	res := Result{Weights: init}
	for k := 0; k < cfg.Iters; k++ {
		a, c := cfg.Schedule.At(k)
		delta := Rademacher(cfg.Seed, k, len(fields))
		plus, minus := w, w
		for i, name := range fields {
			cur := getWeight(w, name)
			step := int32(math.Round(c * float64(delta[i])))
			if step == 0 {
				// c >= 1 makes this unreachable, but rounding a c that landed
				// exactly on a half-integer could still collapse; the guard
				// keeps the two probes distinct no matter what.
				if delta[i] > 0 {
					step = 1
				} else {
					step = -1
				}
			}
			setWeight(&plus, name, clampInt32(int64(cur)+int64(step)))
			setWeight(&minus, name, clampInt32(int64(cur)-int64(step)))
		}

		baseSeed := cfg.Seed + uint64(k)*cfg.Stride
		ev, err := eval.Eval(plus, minus, baseSeed)
		if err != nil {
			return res, fmt.Errorf("iteration %d evaluation: %w", k, err)
		}

		grad := make([]float64, len(fields))
		for i := range fields {
			grad[i] = (ev.WinRate() - 0.5) * 2 / (2 * c * float64(delta[i]))
		}
		next := w
		for i, name := range fields {
			cur := float64(getWeight(w, name))
			setWeight(&next, name, clampInt32(int64(math.Round(cur+a*grad[i]))))
		}

		res.History = append(res.History, Iteration{
			Index: k, A: a, C: c, Result: ev, Plus: plus, Minus: minus, Next: next,
		})
		res.Weights = next
		if cfg.Trace != nil {
			if err := cfg.Trace.WriteRow(k, a, c, ev, next); err != nil {
				return res, fmt.Errorf("writing trace row %d: %w", k, err)
			}
		}
		if cfg.Log != nil {
			fmt.Fprintf(cfg.Log, "iter %d: a=%.4f c=%.0f winrate(plus vs minus)=%.4f games=%d\n",
				k, a, c, ev.WinRate(), ev.Games)
		}
		if bench != nil && cfg.BenchEvery > 0 && (k+1)%cfg.BenchEvery == 0 {
			rate, err := bench(k, next)
			if err != nil {
				return res, fmt.Errorf("bench at iteration %d: %w", k, err)
			}
			if cfg.Log != nil {
				fmt.Fprintf(cfg.Log, "iter %d: bench vs bot winrate=%.4f\n", k, rate)
			}
		}
		w = next
	}
	return res, nil
}

// clampInt32 saturates at the int32 boundary rather than wrapping, so a
// runaway step cannot flip a weight's sign.
func clampInt32(v int64) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}

// weightFieldNames lists the CastWeights fields the fitter can tune, in Go
// struct order (deterministic). All are int32 by construction; a future
// non-int32 field is skipped rather than mis-tuned.
func weightFieldNames() []string {
	t := reflect.TypeOf(Weights{})
	names := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Type.Kind() == reflect.Int32 {
			names = append(names, t.Field(i).Name)
		}
	}
	return names
}

// resolveFitFields turns a -fit comma list into the field names to tune,
// validating every name against the struct and preserving the list's order so
// the Rademacher stream's meaning is stable across runs. An empty list tunes
// every int32 field.
func resolveFitFields(fit []string) ([]string, error) {
	valid := make(map[string]bool)
	for _, n := range weightFieldNames() {
		valid[n] = true
	}
	var out []string
	seen := make(map[string]bool)
	for _, tok := range fit {
		name := strings.TrimSpace(tok)
		if name == "" {
			continue
		}
		if !valid[name] {
			known := weightFieldNames()
			sort.Strings(known)
			return nil, fmt.Errorf("-fit: unknown weight %q (known: %s)", name, strings.Join(known, ", "))
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	if len(out) == 0 {
		return weightFieldNames(), nil
	}
	return out, nil
}

// getWeight reads one int32 field by name. The caller validated the name.
func getWeight(w Weights, name string) int32 {
	v := reflect.ValueOf(w)
	f := v.FieldByName(name)
	return int32(f.Int())
}

// setWeight writes one int32 field by name. The caller validated the name.
func setWeight(w *Weights, name string, val int32) {
	v := reflect.ValueOf(w).Elem()
	f := v.FieldByName(name)
	f.SetInt(int64(val))
}
