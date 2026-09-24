package main

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The -mode ppo plan: control and round 0's eval first, then per round a
// collect (checkpoint r-1 recording its own games on its own seed block), a
// PPO train from checkpoint r-1 on that corpus alone, and an eval of
// checkpoint r on the one fixed block.
func TestPPOPlanStructure(t *testing.T) {
	cfg := mustConfig(t, "-mode", "ppo", "-bin", "/B", "-out", "/O", "-seed-checkpoint", "/S.gpol", "-rounds", "3",
		"-collect-games", "400", "-eval-games", "800")
	st := planPPO(cfg)
	if len(st) != 2+3*3 {
		t.Fatalf("got %d stages, want 11", len(st))
	}
	if st[0].Name != "control" || argValue(t, st[0], "-a") != "bot" || st[1].Name != "eval" || argValue(t, st[1], "-checkpoint") != "/S.gpol" {
		t.Fatalf("head of plan: %+v %+v", st[0], st[1])
	}
	evalSeed := argValue(t, st[0], "-seed")
	collectSeeds := map[string]bool{}
	for _, s := range st[2:] {
		prev := "/S.gpol"
		if s.Gen > 1 {
			prev = filepath.Join("/O", "round"+itoa(s.Gen-1), "ckpt.gpol")
		}
		ckpt := filepath.Join("/O", "round"+itoa(s.Gen), "ckpt.gpol")
		corpus := filepath.Join("/O", "round"+itoa(s.Gen), "corpus.jsonl")
		switch s.Name {
		case "collect":
			if argValue(t, s, "-checkpoint") != prev || argValue(t, s, "-onpolicy-corpus") != corpus ||
				argValue(t, s, "-policynet-kinds") != "attackers,priority" || argValue(t, s, "-games") != "400" {
				t.Fatalf("round %d collect: %v", s.Gen, s.Args)
			}
			seed := argValue(t, s, "-seed")
			if seed == evalSeed || collectSeeds[seed] {
				t.Fatalf("round %d collect seed %s reused", s.Gen, seed)
			}
			collectSeeds[seed] = true
		case "train":
			if argValue(t, s, "-ppo-corpus") != corpus || argValue(t, s, "-init") != prev || argValue(t, s, "-out") != ckpt {
				t.Fatalf("round %d train: %v", s.Gen, s.Args)
			}
		case "eval":
			if argValue(t, s, "-checkpoint") != ckpt || argValue(t, s, "-seed") != evalSeed || argValue(t, s, "-games") != "800" {
				t.Fatalf("round %d eval: %v", s.Gen, s.Args)
			}
		default:
			t.Fatalf("unexpected stage %s", s.Name)
		}
	}
	// The exit-mode plan is untouched by the new flags.
	if renderPlan(plan(mustConfig(t, "-bin", "/B", "-out", "/O", "-gens", "3"))) != renderPlan(plan(mustConfig(t, "-bin", "/B", "-out", "/O", "-gens", "3", "-mode", "exit"))) {
		t.Fatal("-mode exit changed the exit plan")
	}
}

func TestPPOConfigRefusals(t *testing.T) {
	base := []string{"-mode", "ppo", "-bin", "/B", "-out", "/O"}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, "-seed-checkpoint"},
		{[]string{"-seed-checkpoint", "/S", "-collect-games", "300000"}, "seed-stride"},
		{[]string{"-seed-checkpoint", "/S", "-collect-seed", "89999999"}, "overlap the eval block"},
		{[]string{"-seed-checkpoint", "/S", "-rounds", "0"}, "-rounds"},
		{[]string{"-mode", "bogus"}, "-mode"},
	} {
		_, _, err := parseConfig(append(append([]string{}, base...), tc.args...), io.Discard)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: want error containing %q, got %v", tc.args, tc.want, err)
		}
	}
}

func TestPPOSummaryFromFixtures(t *testing.T) {
	ev := func(r float64) evalStats {
		return evalStats{Games: 4000, AWinRate: r, CI: [2]float64{r - 0.015, r + 0.015}}
	}
	var st ppoStats
	st.Examples, st.MeanAdv = 21000, -0.25
	st.ByKind = append(st.ByKind, struct {
		Kind        string  `json:"kind"`
		N           int     `json:"n"`
		Deviated    int     `json:"deviated"`
		DeviatedPct float64 `json:"deviated_pct"`
		MeanAdv     float64 `json:"mean_adv"`
		FinalKL     float64 `json:"final_kl"`
		FinalFlip   float64 `json:"final_flip"`
	}{Kind: "attackers", N: 100, Deviated: 44, DeviatedPct: 44})
	st.Final.KL, st.Final.ClipFrac, st.Final.ValueLogLoss, st.Final.InitValueLogLoss, st.Final.BaseLogLoss = 0.0064, 0.1072, 0.5177, 0.8022, 0.6769
	rows := []ppoRow{{Round: 0, Eval: ev(0.467)}, {Round: 1, Collect: ev(0.4585), Stats: st, Eval: ev(0.48)}, {Round: 2, Collect: ev(0.47), Stats: st, Eval: ev(0.475)}}
	got := ppoSummaryTSV(rows, ev(0.5))
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("summary:\n%s", got)
	}
	want1 := "1\t0.4585\t21000\tattackers 44/100\t44.00\t-0.2500\t0.00640\t0.1072\t0.5177\t0.8022\t0.6769\t0.4800 [0.4650,0.4950] n=4000\t+1.30\t-2.00\tyes\t*\t-"
	if lines[3] != want1 {
		t.Fatalf("round 1 row\n got %q\nwant %q", lines[3], want1)
	}
	if !strings.HasSuffix(lines[4], "\tno\t\t-") || !strings.HasPrefix(lines[2], "0\t-") {
		t.Fatalf("rows:\n%s", got)
	}
	tim := ppoTimingTSV(mustConfig(t, "-mode", "ppo", "-bin", "/B", "-out", "/O", "-seed-checkpoint", "/S", "-collect-games", "400", "-eval-games", "800"),
		[]timing{{Gen: 1, Stage: "collect", Seconds: 7.2}, {Gen: 1, Stage: "train", Seconds: 26}})
	if !strings.Contains(tim, "1\tcollect\t7.2\t16\t2000\t1000000\n") || !strings.Contains(tim, "1\ttrain\t26.0\t1\t0\t-\n") {
		t.Fatalf("timing:\n%s", tim)
	}
}

// TestPPOGateChainsThroughTheIncumbent pins -gate: the plan reads every
// round's play checkpoint from the previous round's next.gpol, and the gate
// stage keeps the incumbent when a round's eval regresses and promotes a
// round that matches or beats it.
func TestPPOGateChainsThroughTheIncumbent(t *testing.T) {
	root := t.TempDir()
	seed := filepath.Join(root, "seed.gpol")
	cfg := mustConfig(t, "-mode", "ppo", "-bin", "/B", "-out", filepath.Join(root, "run"), "-seed-checkpoint", seed, "-rounds", "3", "-gate")
	st := planPPO(cfg)
	gates := 0
	for _, s := range st {
		switch s.Name {
		case "gate":
			gates++
		case "collect", "train":
			want := filepath.Join(roundDir(cfg, s.Gen-1), "next.gpol")
			if s.Needs[len(s.Needs)-1] != want {
				t.Fatalf("round %d %s plays %v, want %s", s.Gen, s.Name, s.Needs, want)
			}
		}
	}
	if gates != 4 {
		t.Fatalf("%d gate stages, want 4 (rounds 0-3)", gates)
	}
	if err := createPPOOut(cfg); err != nil {
		t.Fatal(err)
	}
	write := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(seed, "ckpt0")
	evals := []float64{0.47, 0.46, 0.47, 0.48}
	for r, e := range evals {
		if r > 0 {
			write(roundCkpt(cfg, r), "ckpt"+itoa(r))
		}
		write(filepath.Join(roundDir(cfg, r), "eval.json"), `{"pooled":{"games":10,"a_win_rate":`+strconv.FormatFloat(e, 'f', -1, 64)+`,"a_win_rate_ci":[0,1]}}`)
		if err := gateRound(cfg, r); err != nil {
			t.Fatal(err)
		}
	}
	// round 1 regresses (next = ckpt0), round 2 ties the incumbent (promoted),
	// round 3 beats it.
	for r, want := range []string{"ckpt0", "ckpt0", "ckpt2", "ckpt3"} {
		got, err := os.ReadFile(playCkpt(cfg, r))
		if err != nil || string(got) != want {
			t.Fatalf("round %d next.gpol = %q (%v), want %q", r, got, err, want)
		}
	}
}
