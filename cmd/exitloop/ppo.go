package main

// -mode ppo (ticket pn13): the on-policy self-distillation PPO loop, the gorge
// port of mtgbld's one multi-round recipe that compounded. Per round r:
//
//  1. collect: checkpoint r-1 plays the default bot as the DEPLOYED seat
//     (botbench -a policynet, the same -policynet-kinds as the eval, argmax,
//     seats traded) on round r's own seed block, recording every decision it
//     scored (-onpolicy-corpus);
//  2. train: policytrain -ppo-corpus fine-tunes checkpoint r-1 on that corpus
//     alone (it is on-policy only for r-1) into checkpoint r;
//  3. eval: checkpoint r against the bot on ONE fixed seed block, disjoint
//     from every collection block, the same block as round 0's seed
//     checkpoint and the bot self-control.
//
// The loop keeps going for the fixed round count regardless of the gate (the
// point is the curve) and marks the best-gated checkpoint in summary.tsv.
// With -gate the rounds CHAIN through the gate (mtgbld's never-regress
// rule): a "gate" stage after each eval writes round r's next.gpol, the
// checkpoint round r+1 collects and trains from — checkpoint r when its eval
// is at least the incumbent's, else the incumbent. Every checkpoint is still
// trained and evaluated, so the curve is complete either way.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func roundDir(cfg config, r int) string { return filepath.Join(cfg.out, fmt.Sprintf("round%d", r)) }

// roundCkpt is checkpoint r's path: the seed checkpoint for round 0.
func roundCkpt(cfg config, r int) string {
	if r == 0 {
		return cfg.seedCkpt
	}
	return filepath.Join(roundDir(cfg, r), "ckpt.gpol")
}

func collectSeed(cfg config, r int) uint64 { return cfg.collectSeed + uint64(r-1)*cfg.seedStride }

// playCkpt is the checkpoint round r+1 plays and trains from: checkpoint r
// ungated, round r's gated next.gpol with -gate.
func playCkpt(cfg config, r int) string {
	if cfg.gate {
		return filepath.Join(roundDir(cfg, r), "next.gpol")
	}
	return roundCkpt(cfg, r)
}

// gateRecord is round r's gate.json: its eval, the incumbent it was gated
// against, and the checkpoint round r+1 plays.
type gateRecord struct {
	Round          int     `json:"round"`
	Eval           float64 `json:"eval"`
	Passed         bool    `json:"passed"`
	IncumbentRound int     `json:"incumbent_round"`
	IncumbentEval  float64 `json:"incumbent_eval"`
}

// gateRound is the internal "gate" stage: compare round r's eval with the
// incumbent (round r-1's gate record; round 0 is the first incumbent), write
// gate.json, and copy the winner to next.gpol.
func gateRound(cfg config, r int) error {
	ev, err := readEvalFile(filepath.Join(roundDir(cfg, r), "eval.json"))
	if err != nil {
		return err
	}
	g := gateRecord{Round: r, Eval: ev.AWinRate, Passed: true, IncumbentRound: r, IncumbentEval: ev.AWinRate}
	if r > 0 {
		data, err := os.ReadFile(filepath.Join(roundDir(cfg, r-1), "gate.json"))
		if err != nil {
			return err
		}
		var prev gateRecord
		if err := json.Unmarshal(data, &prev); err != nil {
			return err
		}
		g.Passed = ev.AWinRate >= prev.IncumbentEval
		if !g.Passed {
			g.IncumbentRound, g.IncumbentEval = prev.IncumbentRound, prev.IncumbentEval
		}
	}
	data, err := json.Marshal(g)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(roundDir(cfg, r), "gate.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	ck, err := os.ReadFile(roundCkpt(cfg, g.IncumbentRound))
	if err != nil {
		return err
	}
	return os.WriteFile(playCkpt(cfg, r), ck, 0o644)
}

// planPPO returns every stage of a -mode ppo run in execution order.
func planPPO(cfg config) []stage {
	bin := func(n string) string { return filepath.Join(cfg.bin, n) }
	bench := func(seed uint64, games int) []string {
		return []string{"-pairs", cfg.pairs, "-games", strconv.Itoa(games),
			"-seed", strconv.FormatUint(seed, 10), "-out", "json",
			"-workers", strconv.Itoa(cfg.workers), "-dir", cfg.cards}
	}
	policy := func(ckpt string) []string {
		return []string{"-a", "policynet", "-b", "bot", "-checkpoint", ckpt, "-policynet-kinds", cfg.pnKinds, "-policynet-admission", cfg.admission}
	}
	st := []stage{
		{Name: "control", Gen: -1, Binary: bin("botbench"),
			Args:   append([]string{"-a", "bot", "-b", "bot"}, bench(cfg.evalSeed, cfg.evalGames)...),
			Stdout: filepath.Join(cfg.out, "control.json")},
		{Name: "eval", Gen: 0, Binary: bin("botbench"),
			Args:   append(policy(roundCkpt(cfg, 0)), bench(cfg.evalSeed, cfg.evalGames)...),
			Stdout: filepath.Join(roundDir(cfg, 0), "eval.json"), Needs: []string{roundCkpt(cfg, 0)}},
	}
	gate := func(r int) stage {
		return stage{Name: "gate", Gen: r, Stdout: filepath.Join(roundDir(cfg, r), "gate.json"),
			Needs: []string{filepath.Join(roundDir(cfg, r), "eval.json")}}
	}
	if cfg.gate {
		st = append(st, gate(0))
	}
	for r := 1; r <= cfg.rounds; r++ {
		dir := roundDir(cfg, r)
		prev := playCkpt(cfg, r-1)
		corpus := filepath.Join(dir, "corpus.jsonl")
		ckpt := roundCkpt(cfg, r)
		cargs := append(policy(prev), bench(collectSeed(cfg, r), cfg.collectGames)...)
		cargs = append(cargs, "-onpolicy-corpus", corpus)
		st = append(st, stage{Name: "collect", Gen: r, Binary: bin("botbench"), Args: cargs,
			Stdout: filepath.Join(dir, "collect.json"), Needs: []string{prev}})
		targs := []string{"-ppo-corpus", corpus, "-init", prev, "-out", ckpt, "-stats-out", filepath.Join(dir, "stats.json")}
		targs = append(targs, strings.Fields(cfg.trainArgs)...)
		st = append(st, stage{Name: "train", Gen: r, Binary: bin("policytrain"), Args: targs,
			Stdout: filepath.Join(dir, "train.txt"), Needs: []string{corpus, prev}})
		st = append(st, stage{Name: "eval", Gen: r, Binary: bin("botbench"),
			Args:   append(policy(ckpt), bench(cfg.evalSeed, cfg.evalGames)...),
			Stdout: filepath.Join(dir, "eval.json"), Needs: []string{ckpt}})
		if cfg.gate {
			st = append(st, gate(r))
		}
	}
	return st
}

// validatePPO checks a -mode ppo configuration.
func validatePPO(cfg config) error {
	npairs := uint64(len(strings.Split(cfg.pairs, ",")))
	collectBlock := uint64(cfg.collectGames) * npairs
	evalLo, evalHi := cfg.evalSeed, cfg.evalSeed+uint64(cfg.evalGames)*npairs
	switch {
	case cfg.bin == "" || cfg.out == "":
		return errors.New("-bin and -out are required")
	case cfg.seedCkpt == "":
		return errors.New("-mode ppo needs -seed-checkpoint (round 0's checkpoint)")
	case cfg.rounds < 1 || cfg.collectGames < 1 || cfg.evalGames < 1 || cfg.workers < 1:
		return errors.New("-rounds, -collect-games, -eval-games and -workers must be >= 1")
	case cfg.pnKinds == "":
		return errors.New("-policynet-kinds names no kind")
	case cfg.rounds > 1 && collectBlock > cfg.seedStride:
		return errors.New("-seed-stride is smaller than one round's collection block (collect-games x pairs): rounds would share seeds")
	}
	for r := 1; r <= cfg.rounds; r++ {
		lo := collectSeed(cfg, r)
		if hi := lo + collectBlock; lo < evalHi && evalLo < hi {
			return fmt.Errorf("round %d's collection seeds [%d,%d) overlap the eval block [%d,%d)", r, lo, hi, evalLo, evalHi)
		}
	}
	return nil
}

// createPPOOut refuses an existing -out and creates one directory per round.
func createPPOOut(cfg config) error {
	if _, err := os.Lstat(cfg.out); err == nil {
		return fmt.Errorf("-out %s already exists (the run directory must be new)", cfg.out)
	} else if !os.IsNotExist(err) {
		return err
	}
	for r := 0; r <= cfg.rounds; r++ {
		if err := os.MkdirAll(roundDir(cfg, r), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// ppoStats is the subset of policytrain -stats-out (cmd/policytrain
// PPOResult) the summary reads; same JSON field names.
type ppoStats struct {
	Examples int     `json:"examples"`
	Deviated int     `json:"deviated"`
	MeanAdv  float64 `json:"mean_adv"`
	ByKind   []struct {
		Kind        string  `json:"kind"`
		N           int     `json:"n"`
		Deviated    int     `json:"deviated"`
		DeviatedPct float64 `json:"deviated_pct"`
		MeanAdv     float64 `json:"mean_adv"`
		FinalKL     float64 `json:"final_kl"`
		FinalFlip   float64 `json:"final_flip"`
	} `json:"by_kind"`
	Final struct {
		KL               float64 `json:"kl"`
		ClipFrac         float64 `json:"clip_frac"`
		BaseRate         float64 `json:"base_rate"`
		BaseLogLoss      float64 `json:"base_log_loss"`
		InitValueLogLoss float64 `json:"init_value_log_loss"`
		ValueLogLoss     float64 `json:"value_log_loss"`
	} `json:"final"`
}

func readPPOStats(path string) (ppoStats, error) {
	var s ppoStats
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}

// ppoRow is one round's summary row (round 0 has only Eval).
type ppoRow struct {
	Round   int
	Collect evalStats
	Stats   ppoStats
	Eval    evalStats
	Gate    *gateRecord // nil ungated
}

func collectPPO(cfg config) ([]ppoRow, evalStats, error) {
	control, err := readEvalFile(filepath.Join(cfg.out, "control.json"))
	if err != nil {
		return nil, control, err
	}
	var rows []ppoRow
	for r := 0; r <= cfg.rounds; r++ {
		dir := roundDir(cfg, r)
		row := ppoRow{Round: r}
		if row.Eval, err = readEvalFile(filepath.Join(dir, "eval.json")); err != nil {
			return nil, control, err
		}
		if cfg.gate {
			data, err := os.ReadFile(filepath.Join(dir, "gate.json"))
			if err != nil {
				return nil, control, err
			}
			row.Gate = &gateRecord{}
			if err := json.Unmarshal(data, row.Gate); err != nil {
				return nil, control, err
			}
		}
		if r > 0 {
			if row.Collect, err = readEvalFile(filepath.Join(dir, "collect.json")); err != nil {
				return nil, control, err
			}
			if row.Stats, err = readPPOStats(filepath.Join(dir, "stats.json")); err != nil {
				return nil, control, err
			}
		}
		rows = append(rows, row)
	}
	return rows, control, nil
}

// ppoSummaryTSV is -mode ppo's summary.tsv: the control, then one row per
// checkpoint. Row r's corpus columns describe the corpus checkpoint r was
// TRAINED on (checkpoint r-1's own games, collect_win its win rate there).
// beat_prev compares eval r with eval r-1; best marks the best-gated
// checkpoint (highest eval win rate, ties to the earlier round). Nothing
// reads the clock, so equal runs give equal bytes.
func ppoSummaryTSV(rows []ppoRow, control evalStats) string {
	var b strings.Builder
	hdr := []string{"round", "collect_win", "decisions", "deviations", "attackers_override_pct", "mean_adv",
		"kl", "clip_frac", "value_ll", "value_ll_init", "base_ll", "eval", "d_round0_pp", "d_control_pp", "beat_prev", "best", "gate"}
	b.WriteString(strings.Join(hdr, "\t") + "\n")
	b.WriteString("control\t-\t-\t-\t-\t-\t-\t-\t-\t-\t-\tbot " + fmtEval(control) + "\t-\t-\t-\t-\t-\n")
	best := 0
	for i, r := range rows {
		if r.Eval.AWinRate > rows[best].Eval.AWinRate {
			best = i
		}
	}
	for i, r := range rows {
		f := []string{strconv.Itoa(r.Round)}
		if r.Round == 0 {
			f = append(f, "-", "-", "-", "-", "-", "-", "-", "-", "-", "-")
		} else {
			s := r.Stats
			var dev []string
			atk := "-"
			for _, k := range s.ByKind {
				dev = append(dev, fmt.Sprintf("%s %d/%d", k.Kind, k.Deviated, k.N))
				if k.Kind == "attackers" {
					atk = fmt.Sprintf("%.2f", k.DeviatedPct)
				}
			}
			f = append(f, fmt.Sprintf("%.4f", r.Collect.AWinRate), strconv.Itoa(s.Examples), strings.Join(dev, "; "), atk,
				fmt.Sprintf("%+.4f", s.MeanAdv), fmt.Sprintf("%.5f", s.Final.KL), fmt.Sprintf("%.4f", s.Final.ClipFrac),
				fmt.Sprintf("%.4f", s.Final.ValueLogLoss), fmt.Sprintf("%.4f", s.Final.InitValueLogLoss), fmt.Sprintf("%.4f", s.Final.BaseLogLoss))
		}
		beat := "-"
		if i > 0 {
			beat = "no"
			if r.Eval.AWinRate > rows[i-1].Eval.AWinRate {
				beat = "yes"
			}
		}
		mark := ""
		if i == best {
			mark = "*"
		}
		gate := "-"
		if g := r.Gate; g != nil {
			gate = fmt.Sprintf("next=round%d", g.IncumbentRound)
			if !g.Passed {
				gate = "fail " + gate
			}
		}
		f = append(f, fmtEval(r.Eval), fmt.Sprintf("%+.2f", 100*(r.Eval.AWinRate-rows[0].Eval.AWinRate)),
			fmt.Sprintf("%+.2f", 100*(r.Eval.AWinRate-control.AWinRate)), beat, mark, gate)
		b.WriteString(strings.Join(f, "\t") + "\n")
	}
	return b.String()
}

// ppoTimingTSV is -mode ppo's timing.tsv: per-stage wall seconds and, for
// the bench stages, games/hour.
func ppoTimingTSV(cfg config, ts []timing) string {
	var b strings.Builder
	b.WriteString("round\tstage\tseconds\tworkers\tgames\tgames_per_hour\n")
	npairs := len(strings.Split(cfg.pairs, ","))
	for _, t := range ts {
		games, workers := cfg.evalGames*npairs, cfg.workers
		switch t.Stage {
		case "collect":
			games = cfg.collectGames * npairs
		case "train", "gate":
			games, workers = 0, 1
		}
		gph := "-"
		if games > 0 && t.Seconds > 0 {
			gph = fmt.Sprintf("%.0f", float64(games)*3600/t.Seconds)
		}
		r := strconv.Itoa(t.Gen)
		if t.Gen < 0 {
			r = "control"
		}
		fmt.Fprintf(&b, "%s\t%s\t%.1f\t%d\t%d\t%s\n", r, t.Stage, t.Seconds, workers, games, gph)
	}
	return b.String()
}
