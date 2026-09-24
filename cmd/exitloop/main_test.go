package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/plan-gens3.golden")

func mustConfig(t *testing.T, args ...string) config {
	t.Helper()
	cfg, _, err := parseConfig(args, io.Discard)
	if err != nil {
		t.Fatalf("parseConfig(%v): %v", args, err)
	}
	return cfg
}

func argValue(t *testing.T, s stage, name string) string {
	t.Helper()
	for i, a := range s.Args {
		if a == name && i+1 < len(s.Args) {
			return s.Args[i+1]
		}
	}
	return ""
}

func hasArg(s stage, name string) bool {
	for _, a := range s.Args {
		if a == name {
			return true
		}
	}
	return false
}

// The -gens 3 plan with every default is pinned byte for byte, and its
// structure is asserted: seeds disjoint per generation, generation 0 has no
// prior, generation k points at generation k-1, the eval seed block is the
// same for every generation and the control, and the cumulative corpus grows.
func TestPlanGens3Golden(t *testing.T) {
	cfg := mustConfig(t, "-bin", "/B", "-out", "/O", "-gens", "3")
	st := plan(cfg)
	got := renderPlan(st)
	golden := filepath.Join("testdata", "plan-gens3.golden")
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("plan drifted from %s (rerun with -update if intended):\n%s", golden, got)
	}

	// control + 3 x (teacher, train, 2 eval arms)
	if len(st) != 1+3*4 {
		t.Fatalf("got %d stages, want 13", len(st))
	}
	if st[0].Name != "control" || argValue(t, st[0], "-a") != "bot" || argValue(t, st[0], "-b") != "bot" || hasArg(st[0], "-checkpoint") {
		t.Fatalf("stage 0 is not the bot self-control: %+v", st[0])
	}
	evalSeed := argValue(t, st[0], "-seed")
	seeds := map[string]int{}
	var corpusLens []int
	for _, s := range st {
		switch {
		case s.Name == "teacher":
			seed := argValue(t, s, "-seed")
			if prev, dup := seeds[seed]; dup {
				t.Fatalf("gen %d reuses gen %d's teacher seed %s", s.Gen, prev, seed)
			}
			seeds[seed] = s.Gen
			if s.Gen == 0 {
				if hasArg(s, "-prior-checkpoint") || hasArg(s, "-value-checkpoint") || argValue(t, s, "-horizon") != "0" {
					t.Fatalf("gen 0 teacher is not the unguided game-end teacher: %v", s.Args)
				}
				continue
			}
			prev := filepath.Join("/O", "gen"+itoa(s.Gen-1), "ckpt.gpol")
			if argValue(t, s, "-prior-checkpoint") != prev || argValue(t, s, "-value-checkpoint") != prev {
				t.Fatalf("gen %d teacher does not use %s: %v", s.Gen, prev, s.Args)
			}
			if argValue(t, s, "-horizon") != "2" || argValue(t, s, "-prior-topk") != "5" {
				t.Fatalf("gen %d teacher horizon/topk: %v", s.Gen, s.Args)
			}
			if len(s.Needs) != 1 || s.Needs[0] != prev {
				t.Fatalf("gen %d teacher needs %v, want [%s]", s.Gen, s.Needs, prev)
			}
		case s.Name == "train":
			corpusLens = append(corpusLens, len(strings.Split(argValue(t, s, "-corpus"), ",")))
		case strings.HasPrefix(s.Name, "eval-"):
			if argValue(t, s, "-seed") != evalSeed {
				t.Fatalf("gen %d %s eval seed %s != control %s", s.Gen, s.Name, argValue(t, s, "-seed"), evalSeed)
			}
			want := filepath.Join("/O", "gen"+itoa(s.Gen), "ckpt.gpol")
			if argValue(t, s, "-checkpoint") != want {
				t.Fatalf("gen %d eval checkpoint %s, want %s", s.Gen, argValue(t, s, "-checkpoint"), want)
			}
		}
	}
	if len(seeds) != 3 {
		t.Fatalf("teacher seeds %v", seeds)
	}
	if len(corpusLens) != 3 || corpusLens[0] != 1 || corpusLens[1] != 2 || corpusLens[2] != 3 {
		t.Fatalf("cumulative corpus sizes %v, want [1 2 3]", corpusLens)
	}
}

func itoa(i int) string { return string(rune('0' + i)) }

func TestConfigRefusals(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"-out", "/O"}, "-bin and -out"},
		{[]string{"-bin", "/B", "-out", "/O", "-horizon", "0"}, "-horizon"},
		{[]string{"-bin", "/B", "-out", "/O", "-train-args", "-residual-init 2"}, "value head"},
		{[]string{"-bin", "/B", "-out", "/O", "-train-args", "-value-weight 1 -value-weight=0"}, "value head"},
		{[]string{"-bin", "/B", "-out", "/O", "-seed-stride", "100"}, "seed-stride"},
		{[]string{"-bin", "/B", "-out", "/O", "-eval-kinds", ";"}, "no arm"},
	} {
		_, _, err := parseConfig(tc.args, io.Discard)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: want error containing %q, got %v", tc.args, tc.want, err)
		}
	}
	// One generation needs neither a horizon nor a value head.
	mustConfig(t, "-bin", "/B", "-out", "/O", "-gens", "1", "-horizon", "0", "-train-args", "-epochs 2")
	if !trainsValueHead("-value-weight=0.5") || !trainsValueHead("--value-weight 1") {
		t.Fatal("trainsValueHead misses a positive -value-weight")
	}
}

func TestExistingOutRefused(t *testing.T) {
	dir := t.TempDir()
	var stderr strings.Builder
	code := run([]string{"-bin", "/nonexistent", "-out", dir}, io.Discard, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("exit %d, stderr %q: want an already-exists refusal", code, stderr.String())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("refused run wrote into %s: %v", dir, entries)
	}
}

// A stage that fails aborts the run naming the stage and its stderr tail,
// and the run never reaches a stage whose checkpoint is missing.
func TestStageFailureIsLoud(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\necho 'boom: the reason' >&2\nexit 3\n"
	for _, n := range []string{"botbench", "searchteacher", "policytrain"} {
		if err := os.WriteFile(filepath.Join(bin, n), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(t.TempDir(), "run")
	var stderr strings.Builder
	code := run([]string{"-bin", bin, "-out", out}, io.Discard, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "stage control failed") || !strings.Contains(stderr.String(), "boom: the reason") {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}

	_, err := execStage(stage{Name: "teacher", Gen: 1, Binary: filepath.Join(bin, "searchteacher"),
		Stdout: filepath.Join(out, "gen1", "teacher.txt"), Needs: []string{filepath.Join(out, "gen0", "ckpt.gpol")}})
	if err == nil || !strings.Contains(err.Error(), "missing input") {
		t.Fatalf("want a missing-input refusal, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "gen1", "teacher.txt")); !os.IsNotExist(err) {
		t.Fatalf("a refused stage created its stdout file: %v", err)
	}
}

const teacherFixture = `{"SearchOver":true,"BaseOver":true,"SearchWon":true,"Decisions":[{"Covered":true,"ChosenDiffersFromBot":true},{"Covered":true},{"Covered":false}]}
{"SearchOver":true,"BaseOver":true,"BaseWon":true,"Decisions":[{"Covered":true}]}
{"SearchOver":true,"BaseOver":true,"SearchWon":true,"BaseWon":true}
{"SearchOver":true,"BaseOver":true,"SearchDraw":true,"Decisions":[{"Covered":true,"ChosenDiffersFromBot":true}]}
{"Error":"search: boom","Decisions":[{"Covered":true,"ChosenDiffersFromBot":true}]}
`

func TestReadTeacherPairedDelta(t *testing.T) {
	ts, err := readTeacher(strings.NewReader(teacherFixture))
	if err != nil {
		t.Fatal(err)
	}
	// diffs: +1, -1, 0, +0.5 (a finished draw scores 0.5) -> mean 0.125,
	// sd = sqrt(((.875^2)+(1.125^2)+(.125^2)+(.375^2))/3) = sqrt(2.1875/3).
	if ts.Games != 4 || ts.Errors != 1 {
		t.Fatalf("games %d errors %d", ts.Games, ts.Errors)
	}
	if ts.Asked != 5 || ts.Covered != 4 || ts.Overrides != 2 {
		t.Fatalf("asked %d covered %d overrides %d (errored games must not count)", ts.Asked, ts.Covered, ts.Overrides)
	}
	if !near(ts.Delta, 0.125) || !near(ts.DeltaCI, 1.96*0.853912563829967/2) {
		t.Fatalf("delta %v ± %v", ts.Delta, ts.DeltaCI)
	}
}

func near(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }

const evalFixture = `{"policy_a":"policynet","pairs":[],"pooled":{"pairs":5,"games":1000,"a_win_rate":0.4812,"a_win_rate_ci":[0.4503,0.5122]}}`

const trainFixture = `corpus x: 10 records
value head: hidden 32 weight 1 blend 0; targets on 205 train / 22 holdout examples; base rate (train mean target) 0.5756
holdout per-kind top-1 among labelled options (blended below is just that, blended; loss mode ce):
  (split by game; m-ovr/m-keep are model top-1 on the teacher-override/kept subsets, ovr-n the override count, m-bot the fraction of model picks that are a bot pick)
  kind          model      bot    first   random        n    ovr-n    m-ovr   m-keep    m-bot
  attackers     0.990    0.980    0.750    0.792      400        8    0.125    1.000    0.995
  priority      0.970    0.985    0.417    0.226      300        4    0.000    0.980    1.000
value head holdout (split by game; blend 0; 205 train / 22 holdout examples with a target):
  predictor                log-loss      brier
  value head               0.650000   0.230000
  base rate 0.5756         0.690000   0.250000
train 205 examples, holdout 22
`

func TestSummaryFromFixtures(t *testing.T) {
	ev, err := readEval(strings.NewReader(evalFixture))
	if err != nil {
		t.Fatal(err)
	}
	if ev.Games != 1000 || ev.AWinRate != 0.4812 || ev.CI != [2]float64{0.4503, 0.5122} {
		t.Fatalf("eval %+v", ev)
	}
	if _, err := readEval(strings.NewReader(`{"pooled":{"a_win_rate":0.5}}`)); err == nil {
		t.Fatal("a pooled block without a CI must be refused")
	}
	ho := readHoldout(strings.NewReader(trainFixture))
	wantHo := "attackers m=0.990 b=0.980 n=400 ovr=8 m-ovr=0.125 m-bot=0.995; priority m=0.970 b=0.985 n=300 ovr=4 m-ovr=0.000 m-bot=1.000; value ll=0.650000 base=0.690000"
	if ho != wantHo {
		t.Fatalf("holdout\n got %q\nwant %q", ho, wantHo)
	}
	ts, _ := readTeacher(strings.NewReader(teacherFixture))
	cfg := mustConfig(t, "-bin", "/B", "-out", "/O")
	control := evalStats{Games: 1000, AWinRate: 0.5, CI: [2]float64{0.469, 0.531}}
	got := summaryTSV(cfg, []genRow{{Gen: 0, Labels: 7, Teacher: ts, Holdout: ho, Evals: []evalStats{ev, ev}}}, control)
	want := "gen\tlabels\tteacher_games\tteacher_errors\tasked\tcovered\toverrides\toverride_pct\tpaired_delta_pp\tdelta_ci_pp\teval_attackers\teval_attackers+priority\tholdout\n" +
		"control\t-\t-\t-\t-\t-\t-\t-\t-\t-\tbot 0.5000 [0.4690,0.5310] n=1000\tbot 0.5000 [0.4690,0.5310] n=1000\t-\n" +
		"0\t7\t4\t1\t5\t4\t2\t50.00\t+12.50\t83.68\t0.4812 [0.4503,0.5122] n=1000\t0.4812 [0.4503,0.5122] n=1000\t" + wantHo + "\n"
	if got != want {
		t.Fatalf("summary\n got %q\nwant %q", got, want)
	}

	tim := timingTSV(cfg, []timing{{Gen: -1, Stage: "control", Seconds: 36}, {Gen: 0, Stage: "teacher", Seconds: 1800}, {Gen: 0, Stage: "train", Seconds: 10}})
	wantTim := "gen\tstage\tseconds\tworkers\tgames\tgames_per_hour\n" +
		"control\tcontrol\t36.0\t16\t1000\t100000\n" +
		"0\tteacher\t1800.0\t16\t500\t1000\n" +
		"0\ttrain\t10.0\t1\t0\t-\n"
	if tim != wantTim {
		t.Fatalf("timing\n got %q\nwant %q", tim, wantTim)
	}
}
