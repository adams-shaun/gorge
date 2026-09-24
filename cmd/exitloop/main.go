// exitloop is the L10 expert-iteration loop
// (docs/superpowers/plans/2026-09-19-learned-cast-profile.md, "L10"): a
// deterministic multi-generation run that orchestrates the three separate
// stage commands (cmd/searchteacher, cmd/policytrain, cmd/botbench) as built
// binaries.
//
// Generation 0's teacher is the unguided PIMC teacher (game-end rollouts, no
// checkpoint). Generation k > 0's teacher uses checkpoint k-1 twice: its
// policy head ranks the candidate list (-prior-checkpoint) and its value head
// scores the horizon leaves (-value-checkpoint). Every generation trains on the
// cumulative label corpus gen0..genk, and every checkpoint is evaluated against
// the default bot on ONE fixed seed block (the same seeds for every generation
// and for the bot self-control), so the generations are comparable.
//
// -mode ppo (ppo.go, ticket pn13) is the on-policy alternative: each round
// the previous checkpoint plays the bot as the deployed seat and records its
// own scored decisions (botbench -onpolicy-corpus), policytrain -ppo-corpus
// fine-tunes it on that corpus alone (PPO, or VDWM with -vdwm in
// -ppo-train-args), and the result is evaluated on the same fixed block;
// -gate chains the rounds through mtgbld's never-regress gate.
//
// The loop never changes a stage's behaviour: it only plans argv lists, runs
// them, and reads their machine-readable outputs (the teacher's GameRecord
// JSONL, botbench's -out json) to write summary.tsv. The wall clock is read
// only to report per-stage seconds (timing.tsv and stdout); nothing a label,
// checkpoint or game reads depends on it.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// defaultPairs are the five uw-tempo vs mono-deck pairs the loop measures.
var defaultPairs = []string{
	"uw-tempo:mono-white-equipment",
	"uw-tempo:mono-blue-tempo",
	"uw-tempo:mono-black-aggro",
	"uw-tempo:mono-red-prowess",
	"uw-tempo:mono-green-stompy",
}

// config is the parsed flag set; plan is a pure function of it.
type config struct {
	bin, out         string
	gens             int
	pairs            string
	games            int
	seed, seedStride uint64
	kinds            string
	oracle           bool
	horizon          int
	priorTopK        int
	trainArgs        string
	evalSeed         uint64
	evalGames        int
	evalKinds        []string
	workers          int
	cards            string

	// -mode ppo (ppo.go): the on-policy PPO loop's own knobs.
	mode         string
	rounds       int
	seedCkpt     string
	collectGames int
	collectSeed  uint64
	pnKinds      string
	admission    string
	gate         bool
	// ticket pn14: stochastic collection (botbench -policynet-temperature)
	// on a linear schedule from collectTemp (round 1) to collectTempFinal
	// (the last round; <= 0 keeps collectTemp throughout), and the collection
	// opponent mix (botbench -opp-mix). Eval is always greedy against -b bot.
	collectTemp      float64
	collectTempFinal float64
	oppMix           string
}

// stage is one command the loop runs: the binary, its argv, the file its
// stdout is captured to (stderr goes to the same path with a .log suffix),
// and the files that must exist before it may start.
type stage struct {
	Name   string
	Gen    int // -1 for the control
	Binary string
	Args   []string
	Stdout string
	Needs  []string
}

// Log is the stage's stderr capture path.
func (s stage) Log() string {
	return strings.TrimSuffix(s.Stdout, filepath.Ext(s.Stdout)) + ".log"
}

func genDir(cfg config, k int) string { return filepath.Join(cfg.out, fmt.Sprintf("gen%d", k)) }

// armName turns an eval-kinds entry into a file-name-safe arm name.
func armName(kinds string) string { return strings.ReplaceAll(kinds, ",", "+") }

// plan returns every stage of the run in execution order: the bot
// self-control once, then per generation the teacher, the trainer and one
// botbench arm per -eval-kinds entry.
func plan(cfg config) []stage {
	bin := func(n string) string { return filepath.Join(cfg.bin, n) }
	evalBase := func() []string {
		return []string{"-pairs", cfg.pairs, "-games", strconv.Itoa(cfg.evalGames),
			"-seed", strconv.FormatUint(cfg.evalSeed, 10), "-out", "json",
			"-workers", strconv.Itoa(cfg.workers), "-dir", cfg.cards}
	}
	var st []stage
	st = append(st, stage{Name: "control", Gen: -1, Binary: bin("botbench"),
		Args:   append([]string{"-a", "bot", "-b", "bot"}, evalBase()...),
		Stdout: filepath.Join(cfg.out, "control.json")})
	var corpus []string
	for k := 0; k < cfg.gens; k++ {
		dir := genDir(cfg, k)
		labels := filepath.Join(dir, "labels.jsonl")
		ckpt := filepath.Join(dir, "ckpt.gpol")
		horizon := 0
		var needs, extra []string
		if k > 0 {
			prev := filepath.Join(genDir(cfg, k-1), "ckpt.gpol")
			horizon = cfg.horizon
			needs = []string{prev}
			extra = []string{"-prior-checkpoint", prev, "-prior-topk", strconv.Itoa(cfg.priorTopK), "-value-checkpoint", prev}
		}
		targs := []string{"-pairs", cfg.pairs, "-games", strconv.Itoa(cfg.games),
			"-seed", strconv.FormatUint(cfg.seed+uint64(k)*cfg.seedStride, 10),
			"-kinds", cfg.kinds, "-oracle=" + strconv.FormatBool(cfg.oracle),
			"-horizon", strconv.Itoa(horizon), "-workers", strconv.Itoa(cfg.workers), "-cards", cfg.cards,
			"-out", filepath.Join(dir, "games.jsonl"), "-labels", labels}
		st = append(st, stage{Name: "teacher", Gen: k, Binary: bin("searchteacher"),
			Args: append(targs, extra...), Stdout: filepath.Join(dir, "teacher.txt"), Needs: needs})
		corpus = append(corpus, labels)
		trargs := []string{"-corpus", strings.Join(corpus, ","), "-out", ckpt}
		trargs = append(trargs, strings.Fields(cfg.trainArgs)...)
		st = append(st, stage{Name: "train", Gen: k, Binary: bin("policytrain"),
			Args: trargs, Stdout: filepath.Join(dir, "train.txt"), Needs: append([]string(nil), corpus...)})
		for _, arm := range cfg.evalKinds {
			args := append([]string{"-a", "policynet", "-b", "bot", "-checkpoint", ckpt, "-policynet-kinds", arm}, evalBase()...)
			st = append(st, stage{Name: "eval-" + armName(arm), Gen: k, Binary: bin("botbench"),
				Args: args, Stdout: filepath.Join(dir, "eval-"+armName(arm)+".json"), Needs: []string{ckpt}})
		}
	}
	return st
}

// renderPlan is the -plan output (and the golden the tests pin).
func renderPlan(st []stage) string {
	var b strings.Builder
	for _, s := range st {
		fmt.Fprintf(&b, "[gen %d] %s: %s %s > %s\n", s.Gen, s.Name, s.Binary, strings.Join(s.Args, " "), s.Stdout)
		if len(s.Needs) > 0 {
			fmt.Fprintf(&b, "  needs %s\n", strings.Join(s.Needs, " "))
		}
	}
	return b.String()
}

func parseConfig(args []string, stderr io.Writer) (config, bool, error) {
	fs := flag.NewFlagSet("exitloop", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var cfg config
	fs.StringVar(&cfg.bin, "bin", "", "directory holding the searchteacher, policytrain and botbench binaries (required)")
	fs.StringVar(&cfg.out, "out", "", "run directory (required; must not exist, created)")
	fs.IntVar(&cfg.gens, "gens", 3, "generations")
	fs.StringVar(&cfg.pairs, "pairs", strings.Join(defaultPairs, ","), "comma list of a:b deck pairs (teacher and eval)")
	fs.IntVar(&cfg.games, "games", 100, "teacher games per pair per generation")
	fs.Uint64Var(&cfg.seed, "seed", 70_000_000, "teacher base seed (generation k uses seed + k*seed-stride)")
	fs.Uint64Var(&cfg.seedStride, "seed-stride", 1_000_000, "teacher seed stride between generations")
	fs.StringVar(&cfg.kinds, "kinds", "attackers,cast", "teacher decision kinds")
	fs.BoolVar(&cfg.oracle, "oracle", true, "clairvoyant teacher (the searchteacher -oracle ceiling)")
	fs.IntVar(&cfg.horizon, "horizon", 2, "rollout horizon for generations k > 0 (generation 0 is the unguided game-end teacher)")
	fs.IntVar(&cfg.priorTopK, "prior-topk", 5, "generations k > 0: non-bot candidates the checkpoint-k-1 prior keeps")
	fs.StringVar(&cfg.trainArgs, "train-args", "-residual-init 2 -holdout-by game -value-weight 1", "extra policytrain arguments (whitespace separated)")
	fs.Uint64Var(&cfg.evalSeed, "eval-seed", 90_000_000, "fixed eval seed block (identical for every generation and the control)")
	fs.IntVar(&cfg.evalGames, "eval-games", 200, "eval games per pair")
	evalKinds := fs.String("eval-kinds", "attackers;attackers,priority", "semicolon list of botbench -policynet-kinds values, one eval arm each")
	fs.IntVar(&cfg.workers, "workers", 16, "parallel games for the teacher and every botbench stage (policytrain is single-threaded)")
	fs.StringVar(&cfg.cards, "cards", ".cards", "compiled corpus directory passed to every stage")
	planOnly := fs.Bool("plan", false, "print the plan and exit")
	fs.StringVar(&cfg.mode, "mode", "exit", "loop: exit (expert iteration: teacher labels -> policytrain -> eval) or ppo (on-policy PPO: the checkpoint's own games -> policytrain -ppo-corpus -> eval; ticket pn13)")
	fs.IntVar(&cfg.rounds, "rounds", 6, "ppo mode: PPO rounds after the seed checkpoint")
	fs.StringVar(&cfg.seedCkpt, "seed-checkpoint", "", "ppo mode: round 0's checkpoint (required)")
	fs.IntVar(&cfg.collectGames, "collect-games", 100, "ppo mode: on-policy games per pair per round")
	fs.Uint64Var(&cfg.collectSeed, "collect-seed", 110_000_000, "ppo mode: collection base seed (round r uses collect-seed + (r-1)*seed-stride)")
	fs.BoolVar(&cfg.gate, "gate", false, "ppo mode: gate every round (mtgbld's never-regress rule): round r+1 collects from and trains from checkpoint r only when its eval is >= the incumbent's, else from the incumbent; every checkpoint is still evaluated")
	fs.StringVar(&cfg.admission, "policynet-admission", "auto", "ppo mode: the deployed seat's subset admission vote (botbench -policynet-admission: auto or sign), for collection and eval alike")
	fs.StringVar(&cfg.pnKinds, "policynet-kinds", "attackers,priority", "ppo mode: the deployed seat's scored kinds, for collection and eval alike")
	fs.Float64Var(&cfg.collectTemp, "collect-temp", 0, "ppo mode (pn14): sample the collection seat at this temperature (botbench -policynet-temperature); 0 = greedy collection (pn13)")
	fs.Float64Var(&cfg.collectTempFinal, "collect-temp-final", 0, "ppo mode (pn14): anneal the collection temperature linearly from -collect-temp (round 1) to this value (the last round); 0 = constant")
	fs.StringVar(&cfg.oppMix, "opp-mix", "", "ppo mode (pn14): collection-only opponent mix, botbench -opp-mix (e.g. explore:0.3)")
	ppoTrain := fs.String("ppo-train-args", "-epochs 4 -lr 0.05 -batch 64 -value-weight 0.5 -ppo-clip 0.2 -ppo-kl 0.1", "ppo mode: extra policytrain arguments (whitespace separated)")
	if err := fs.Parse(args); err != nil {
		return cfg, false, err
	}
	if cfg.mode == "ppo" {
		cfg.trainArgs = *ppoTrain
		return cfg, *planOnly, validatePPO(cfg)
	}
	if cfg.mode != "exit" {
		return cfg, false, fmt.Errorf("-mode %q (want exit or ppo)", cfg.mode)
	}
	for _, k := range strings.Split(*evalKinds, ";") {
		if k = strings.TrimSpace(k); k != "" {
			cfg.evalKinds = append(cfg.evalKinds, k)
		}
	}
	switch {
	case cfg.bin == "" || cfg.out == "":
		return cfg, false, errors.New("-bin and -out are required")
	case cfg.gens < 1 || cfg.games < 1 || cfg.evalGames < 1 || cfg.workers < 1:
		return cfg, false, errors.New("-gens, -games, -eval-games and -workers must be >= 1")
	case len(cfg.evalKinds) == 0:
		return cfg, false, errors.New("-eval-kinds names no arm")
	case cfg.gens > 1 && cfg.horizon < 1:
		// searchteacher refuses -value-checkpoint at -horizon 0; say so here
		// instead of after generation 0 has spent its teacher budget.
		return cfg, false, errors.New("-horizon must be >= 1: generations k > 0 score horizon leaves with checkpoint k-1's value head")
	case cfg.gens > 1 && !trainsValueHead(cfg.trainArgs):
		return cfg, false, errors.New("-train-args must train a value head (-value-weight > 0): generations k > 0 pass checkpoint k-1 as -value-checkpoint")
	case cfg.gens > 1 && uint64(cfg.games)*uint64(len(strings.Split(cfg.pairs, ","))) > cfg.seedStride:
		return cfg, false, errors.New("-seed-stride is smaller than one generation's seed block (games x pairs): generations would share seeds")
	}
	return cfg, *planOnly, nil
}

// trainsValueHead reports whether the policytrain arguments set
// -value-weight to a positive value (the last occurrence wins, as flag does).
func trainsValueHead(args string) bool {
	f := strings.Fields(args)
	w := 0.0
	for i, a := range f {
		name, val, eq := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if name != "value-weight" || !strings.HasPrefix(a, "-") {
			continue
		}
		if !eq {
			if i+1 >= len(f) {
				return false
			}
			val = f[i+1]
		}
		v, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return false
		}
		w = v
	}
	return w > 0
}

func run(args []string, stdout, stderr io.Writer) int {
	cfg, planOnly, err := parseConfig(args, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "exitloop:", err)
		return 2
	}
	if cfg.mode == "ppo" {
		return runPPOLoop(cfg, planOnly, stdout, stderr)
	}
	st := plan(cfg)
	if planOnly {
		fmt.Fprint(stdout, renderPlan(st))
		return 0
	}
	if err := createOut(cfg); err != nil {
		fmt.Fprintln(stderr, "exitloop:", err)
		return 1
	}
	if err := os.WriteFile(filepath.Join(cfg.out, "plan.txt"), []byte(renderPlan(st)), 0o644); err != nil {
		fmt.Fprintln(stderr, "exitloop:", err)
		return 1
	}
	var timings []timing
	for _, s := range st {
		fmt.Fprintf(stderr, "exitloop: [gen %d] %s ...\n", s.Gen, s.Name)
		secs, err := execStage(s)
		if err != nil {
			fmt.Fprintln(stderr, "exitloop:", err)
			return 1
		}
		fmt.Fprintf(stderr, "exitloop: [gen %d] %s done in %.1fs\n", s.Gen, s.Name, secs)
		timings = append(timings, timing{Gen: s.Gen, Stage: s.Name, Seconds: secs})
	}
	rows, control, err := collect(cfg)
	if err != nil {
		fmt.Fprintln(stderr, "exitloop:", err)
		return 1
	}
	tsv := summaryTSV(cfg, rows, control)
	if err := os.WriteFile(filepath.Join(cfg.out, "summary.tsv"), []byte(tsv), 0o644); err != nil {
		fmt.Fprintln(stderr, "exitloop:", err)
		return 1
	}
	tim := timingTSV(cfg, timings)
	if err := os.WriteFile(filepath.Join(cfg.out, "timing.tsv"), []byte(tim), 0o644); err != nil {
		fmt.Fprintln(stderr, "exitloop:", err)
		return 1
	}
	fmt.Fprint(stdout, tsv)
	fmt.Fprintln(stdout)
	fmt.Fprint(stdout, tim)
	return 0
}

// createOut refuses an existing -out and creates it with one directory per
// generation (searchteacher's -labels needs its parent to exist).
func createOut(cfg config) error {
	if _, err := os.Lstat(cfg.out); err == nil {
		return fmt.Errorf("-out %s already exists (the run directory must be new)", cfg.out)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(cfg.out, 0o755); err != nil {
		return err
	}
	for k := 0; k < cfg.gens; k++ {
		if err := os.Mkdir(genDir(cfg, k), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// execStage runs one stage with stdout and stderr captured to files. It
// refuses to start on a missing input (never continue a generation on a
// missing checkpoint) and fails naming the stage and its stderr tail.
func execStage(s stage) (float64, error) {
	for _, n := range s.Needs {
		if _, err := os.Stat(n); err != nil {
			return 0, fmt.Errorf("[gen %d] %s: missing input %s: %v", s.Gen, s.Name, n, err)
		}
	}
	out, err := os.OpenFile(s.Stdout, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return 0, fmt.Errorf("[gen %d] %s: %w", s.Gen, s.Name, err)
	}
	defer out.Close()
	logf, err := os.OpenFile(s.Log(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return 0, fmt.Errorf("[gen %d] %s: %w", s.Gen, s.Name, err)
	}
	defer logf.Close()
	cmd := exec.Command(s.Binary, s.Args...)
	cmd.Stdout = out
	cmd.Stderr = logf
	t0 := time.Now()
	runErr := cmd.Run()
	secs := time.Since(t0).Seconds()
	if runErr != nil {
		return secs, fmt.Errorf("[gen %d] stage %s failed: %v\n--- stderr tail (%s) ---\n%s", s.Gen, s.Name, runErr, s.Log(), tail(s.Log(), 20))
	}
	return secs, nil
}

func tail(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "(unreadable: " + err.Error() + ")"
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// gameRecord is the subset of cmd/searchteacher's GameRecord the summary
// reads (same JSON field names; the teacher's type is in a package main and
// cannot be imported).
type gameRecord struct {
	SearchOver, BaseOver bool
	SearchDraw, BaseDraw bool
	SearchWon, BaseWon   bool
	Error                string
	Decisions            []struct {
		Covered              bool
		ChosenDiffersFromBot bool
	}
}

// teacherStats is one generation's teacher readout, computed exactly as
// searchteacher's summarize does: per-game score 1 / 0.5 (a finished draw) /
// 0, errored games skipped, paired delta = mean(search - base) with a
// 1.96*sd/sqrt(n) half-width.
type teacherStats struct {
	Games, Errors  int
	Delta, DeltaCI float64
	Asked, Covered int
	Overrides      int
}

func score(over, draw, won bool) float64 {
	switch {
	case won:
		return 1
	case over && draw:
		return 0.5
	}
	return 0
}

func readTeacher(r io.Reader) (teacherStats, error) {
	var ts teacherStats
	var diffs []float64
	dec := json.NewDecoder(r)
	for {
		var g gameRecord
		if err := dec.Decode(&g); err == io.EOF {
			break
		} else if err != nil {
			return ts, err
		}
		if g.Error != "" {
			ts.Errors++
			continue
		}
		ts.Games++
		diffs = append(diffs, score(g.SearchOver, g.SearchDraw, g.SearchWon)-score(g.BaseOver, g.BaseDraw, g.BaseWon))
		for _, d := range g.Decisions {
			ts.Asked++
			if d.Covered {
				ts.Covered++
				if d.ChosenDiffersFromBot {
					ts.Overrides++
				}
			}
		}
	}
	m, sd := meanSD(diffs)
	ts.Delta = m
	if n := len(diffs); n > 0 {
		ts.DeltaCI = 1.96 * sd / math.Sqrt(float64(n))
	}
	return ts, nil
}

func meanSD(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	m := s / float64(len(xs))
	if len(xs) < 2 {
		return m, 0
	}
	v := 0.0
	for _, x := range xs {
		v += (x - m) * (x - m)
	}
	return m, math.Sqrt(v / float64(len(xs)-1))
}

// evalStats is botbench -out json's pooled readout.
type evalStats struct {
	Games    int
	AWinRate float64
	CI       [2]float64
}

func readEval(r io.Reader) (evalStats, error) {
	var doc struct {
		Pooled struct {
			Games      int       `json:"games"`
			AWinRate   float64   `json:"a_win_rate"`
			AWinRateCI []float64 `json:"a_win_rate_ci"`
		} `json:"pooled"`
	}
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return evalStats{}, err
	}
	if len(doc.Pooled.AWinRateCI) != 2 {
		return evalStats{}, fmt.Errorf("pooled a_win_rate_ci has %d elements, want 2", len(doc.Pooled.AWinRateCI))
	}
	return evalStats{Games: doc.Pooled.Games, AWinRate: doc.Pooled.AWinRate, CI: [2]float64{doc.Pooled.AWinRateCI[0], doc.Pooled.AWinRateCI[1]}}, nil
}

// readHoldout extracts policytrain's holdout readout from its stdout: the
// per-kind table rows (model/bot top-1, n, override count, model top-1 on the
// overrides, model-picks-bot share) and the value head's log loss against the
// base rate. policytrain has no machine-readable output, so this reads the
// fixed table it prints; an unrecognised table yields "-".
func readHoldout(r io.Reader) string {
	var kinds []string
	var value string
	inTable := false
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		f := strings.Fields(line)
		switch {
		case strings.HasPrefix(line, "holdout per-kind top-1"):
			inTable = true
			continue
		case strings.HasPrefix(line, "value head holdout"):
			inTable = false
		}
		if inTable && len(f) == 10 && f[0] != "kind" {
			// kind model bot first random n ovr-n m-ovr m-keep m-bot
			kinds = append(kinds, fmt.Sprintf("%s m=%s b=%s n=%s ovr=%s m-ovr=%s m-bot=%s", f[0], f[1], f[2], f[5], f[6], f[7], f[9]))
			continue
		}
		if inTable && len(f) > 0 && f[0] != "kind" && !strings.HasPrefix(line, "  (") {
			inTable = false
		}
		if len(f) == 4 && f[0] == "value" && f[1] == "head" {
			value = "value ll=" + f[2]
		}
		if len(f) == 5 && f[0] == "base" && f[1] == "rate" && value != "" {
			value += " base=" + f[3]
		}
	}
	if value != "" {
		kinds = append(kinds, value)
	}
	if len(kinds) == 0 {
		return "-"
	}
	return strings.Join(kinds, "; ")
}

// genRow is one generation's summary row.
type genRow struct {
	Gen     int
	Labels  int
	Teacher teacherStats
	Holdout string
	Evals   []evalStats // one per cfg.evalKinds entry
}

func countLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return bytes.Count(data, []byte("\n")), nil
}

func collect(cfg config) ([]genRow, evalStats, error) {
	control, err := readEvalFile(filepath.Join(cfg.out, "control.json"))
	if err != nil {
		return nil, control, err
	}
	var rows []genRow
	for k := 0; k < cfg.gens; k++ {
		dir := genDir(cfg, k)
		row := genRow{Gen: k}
		if row.Labels, err = countLines(filepath.Join(dir, "labels.jsonl")); err != nil {
			return nil, control, err
		}
		f, err := os.Open(filepath.Join(dir, "games.jsonl"))
		if err != nil {
			return nil, control, err
		}
		row.Teacher, err = readTeacher(f)
		f.Close()
		if err != nil {
			return nil, control, fmt.Errorf("gen %d games.jsonl: %w", k, err)
		}
		tf, err := os.Open(filepath.Join(dir, "train.txt"))
		if err != nil {
			return nil, control, err
		}
		row.Holdout = readHoldout(tf)
		tf.Close()
		for _, arm := range cfg.evalKinds {
			ev, err := readEvalFile(filepath.Join(dir, "eval-"+armName(arm)+".json"))
			if err != nil {
				return nil, control, err
			}
			row.Evals = append(row.Evals, ev)
		}
		rows = append(rows, row)
	}
	return rows, control, nil
}

func readEvalFile(path string) (evalStats, error) {
	f, err := os.Open(path)
	if err != nil {
		return evalStats{}, err
	}
	defer f.Close()
	ev, err := readEval(f)
	if err != nil {
		return ev, fmt.Errorf("%s: %w", path, err)
	}
	return ev, nil
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}

func fmtEval(e evalStats) string {
	return fmt.Sprintf("%.4f [%.4f,%.4f] n=%d", e.AWinRate, e.CI[0], e.CI[1], e.Games)
}

// summaryTSV is summary.tsv: one row per generation plus the control row.
// Deterministic: nothing in it reads the clock (the stage wall times are in
// timing.tsv), so two runs of the same flags produce the same bytes.
func summaryTSV(cfg config, rows []genRow, control evalStats) string {
	var b strings.Builder
	hdr := []string{"gen", "labels", "teacher_games", "teacher_errors", "asked", "covered", "overrides", "override_pct", "paired_delta_pp", "delta_ci_pp"}
	for _, arm := range cfg.evalKinds {
		hdr = append(hdr, "eval_"+armName(arm))
	}
	hdr = append(hdr, "holdout")
	b.WriteString(strings.Join(hdr, "\t") + "\n")
	ctl := []string{"control", "-", "-", "-", "-", "-", "-", "-", "-", "-"}
	for range cfg.evalKinds {
		ctl = append(ctl, "bot "+fmtEval(control))
	}
	ctl = append(ctl, "-")
	b.WriteString(strings.Join(ctl, "\t") + "\n")
	for _, r := range rows {
		t := r.Teacher
		f := []string{strconv.Itoa(r.Gen), strconv.Itoa(r.Labels), strconv.Itoa(t.Games), strconv.Itoa(t.Errors),
			strconv.Itoa(t.Asked), strconv.Itoa(t.Covered), strconv.Itoa(t.Overrides),
			fmt.Sprintf("%.2f", pct(t.Overrides, t.Covered)),
			fmt.Sprintf("%+.2f", 100*t.Delta), fmt.Sprintf("%.2f", 100*t.DeltaCI)}
		for _, e := range r.Evals {
			f = append(f, fmtEval(e))
		}
		f = append(f, r.Holdout)
		b.WriteString(strings.Join(f, "\t") + "\n")
	}
	return b.String()
}

// timing is one stage's measured wall clock.
type timing struct {
	Gen     int
	Stage   string
	Seconds float64
}

// timingTSV is timing.tsv: per-stage wall seconds, the worker count and,
// for teacher and eval stages, games/hour (the teacher plays each seed
// twice -- the search game and its bot twin -- and counts once here).
func timingTSV(cfg config, ts []timing) string {
	var b strings.Builder
	b.WriteString("gen\tstage\tseconds\tworkers\tgames\tgames_per_hour\n")
	npairs := len(strings.Split(cfg.pairs, ","))
	for _, t := range ts {
		games, workers := 0, cfg.workers
		switch {
		case t.Stage == "teacher":
			games = cfg.games * npairs
		case t.Stage == "train":
			workers = 1
		default:
			games = cfg.evalGames * npairs
		}
		gph := "-"
		if games > 0 && t.Seconds > 0 {
			gph = fmt.Sprintf("%.0f", float64(games)*3600/t.Seconds)
		}
		gen := strconv.Itoa(t.Gen)
		if t.Gen < 0 {
			gen = "control"
		}
		fmt.Fprintf(&b, "%s\t%s\t%.1f\t%d\t%d\t%s\n", gen, t.Stage, t.Seconds, workers, games, gph)
	}
	return b.String()
}

// runPPOLoop is run for -mode ppo.
func runPPOLoop(cfg config, planOnly bool, stdout, stderr io.Writer) int {
	st := planPPO(cfg)
	if planOnly {
		fmt.Fprint(stdout, renderPlan(st))
		return 0
	}
	fail := func(err error) int {
		fmt.Fprintln(stderr, "exitloop:", err)
		return 1
	}
	if err := createPPOOut(cfg); err != nil {
		return fail(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.out, "plan.txt"), []byte(renderPlan(st)), 0o644); err != nil {
		return fail(err)
	}
	var timings []timing
	for _, s := range st {
		fmt.Fprintf(stderr, "exitloop: [round %d] %s ...\n", s.Gen, s.Name)
		var secs float64
		var err error
		if s.Name == "gate" {
			err = gateRound(cfg, s.Gen)
		} else {
			secs, err = execStage(s)
		}
		if err != nil {
			return fail(err)
		}
		fmt.Fprintf(stderr, "exitloop: [round %d] %s done in %.1fs\n", s.Gen, s.Name, secs)
		timings = append(timings, timing{Gen: s.Gen, Stage: s.Name, Seconds: secs})
		// timing.tsv is rewritten after every stage so a long run's progress
		// is readable before it ends.
		_ = os.WriteFile(filepath.Join(cfg.out, "timing.tsv"), []byte(ppoTimingTSV(cfg, timings)), 0o644)
	}
	rows, control, err := collectPPO(cfg)
	if err != nil {
		return fail(err)
	}
	tsv := ppoSummaryTSV(rows, control)
	if err := os.WriteFile(filepath.Join(cfg.out, "summary.tsv"), []byte(tsv), 0o644); err != nil {
		return fail(err)
	}
	tim := ppoTimingTSV(cfg, timings)
	fmt.Fprint(stdout, tsv)
	fmt.Fprintln(stdout)
	fmt.Fprint(stdout, tim)
	return 0
}
