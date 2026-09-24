package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/policynet"
)

// cmd/policytrain trains the per-option scorer (plan L9b) on the
// search-teacher label corpus (cmd/searchteacher -labels) and writes a
// versioned checkpoint. Deterministic single-threaded SGD: same seed and
// same corpus ⇒ byte-identical checkpoint (TestTrainerDeterministic). There
// is deliberately NO -workers flag: sharding the update is a data race
// (Hogwild) and sharding the forward pass alone cannot pay for its
// deterministic reduce at this corpus size — see train.go's header.
//
// -ppo-corpus switches to the on-policy mode (ticket pn13, ppo.go): one PPO
// (or, with -vdwm, VDWM) round fine-tuning -init on its own recorded games
// (cmd/botbench -onpolicy-corpus), with -stats-out the round's JSON readout.
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("policytrain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		corpora        = fs.String("corpus", "", "comma-separated label-corpus JSONL paths (plain or gzip)")
		out            = fs.String("out", "", "checkpoint output path (required)")
		epochs         = fs.Int("epochs", 30, "training epochs")
		lr             = fs.Float64("lr", 0.1, "SGD learning rate (per-example, scaled by 1/batch)")
		batch          = fs.Int("batch", 64, "batch size")
		seed           = fs.Int64("seed", 1, "run seed: split, shuffles and initialisation")
		holdout        = fs.Float64("holdout", 0.1, "holdout fraction (never updated on)")
		holdoutBy      = fs.String("holdout-by", HoldoutByExample, "holdout split unit: example (shuffle decisions; one game's decisions can land on both sides) or game (hold out whole games by pair/seed/game index, so no held-out board state is trained on)")
		embed          = fs.Int("embed", 128, "H, the shared embedding width")
		hidden         = fs.Int("hidden", 128, "hidden layer width")
		rankWeight     = fs.Float64("rank-weight", policynet.DefaultRankWeight, "weight of the ranking term against the value term (in CE mode the plain CE weight; in hybrid mode scaled by the per-example margin). INERT in pure CE mode: the per-batch gradient clip normalises any uniform loss scale away — tune -lr and -clip instead")
		huberDelta     = fs.Float64("huber-delta", policynet.DefaultHuberDelta, "Huber transition point (teacher values live in [0,1])")
		overrideWeight = fs.Float64("override-weight", 1, "multiplier on examples where the teacher overrode the bot (1 = no reweighting, >1 up-weights the informative decisions)")
		lossMode       = fs.String("loss", "ce", "loss mode: ce (pure argmax cross-entropy, the value term off), hybrid (value + margin-weighted rank) or value (value only)")
		clip           = fs.Float64("clip", 1, "global L2 gradient-norm cap per batch, on the summed batch gradient before the lr/batch scale (<= 0 disables; uncapped CE divergence is rejected with an error and no checkpoint)")
		residualInit   = fs.Float64("residual-init", 0, "fixed bot-prior residual weight: options in the bot's own answer score this much higher (0 disables; a positive value starts the model at the bot baseline)")
		valueHidden    = fs.Int("value-hidden", 32, "value head hidden width (used only when -value-weight > 0)")
		valueWeight    = fs.Float64("value-weight", 0, "weight of the value head's BCE term on the game outcome (0 = no value head; the run is then bit-identical to a pre-value-head trainer)")
		valueBlend     = fs.Float64("value-blend", 0, "value target blend b in [0,1]: (1-b)*game outcome + b*teacher-chosen candidate's rollout mean")
		ppoCorpora     = fs.String("ppo-corpus", "", "comma-separated ON-POLICY corpus paths (cmd/botbench -onpolicy-corpus): switches to the PPO mode (ticket pn13), which fine-tunes -init on its own games; -corpus must then be empty")
		initCkpt       = fs.String("init", "", "PPO mode: the checkpoint that played the -ppo-corpus games (required; training starts from it)")
		ppoClip        = fs.Float64("ppo-clip", 0.2, "PPO mode: surrogate clip epsilon")
		ppoKL          = fs.Float64("ppo-kl", 0.1, "PPO mode: KL(pi_old || pi) anchor weight")
		ppoAdvNorm     = fs.Bool("ppo-adv-norm", true, "PPO mode: normalise advantages per batch")
		ppoBaseline    = fs.String("ppo-baseline", BaselineValue, "PPO mode: advantage baseline, value (outcome - V_old(s) from the init's value head) or mean (outcome - train mean outcome)")
		ppoKinds       = fs.String("ppo-kinds", "", "PPO mode: comma list of decision kinds the policy term trains (empty = every recorded kind); other kinds still train the value head")
		vdwm           = fs.Bool("vdwm", false, "PPO mode: train the value-disagreement-weighted margin loss (VDWM) on the on-policy corpus instead of the PPO objective (weight (1 - p_old)*|outcome - baseline|, normalised per batch; -ppo-clip/-ppo-kl/-ppo-adv-norm unused)")
		vdwmMargin     = fs.Float64("vdwm-margin", 1, "PPO mode with -vdwm: the hinge margin m")
		statsOut       = fs.String("stats-out", "", "PPO mode: write the round's machine-readable readout (JSON) to this path")
		kindLoss       = fs.String("kind-loss", "attackers=bce", "per-kind loss overrides as kind=mode,... (e.g. attackers=bce); kinds not listed keep -loss. The attackers default is bce: a per-option binary logistic loss trains the score LEVEL the seat's per-option admission rule reads, which argmax CE (shift-invariant) cannot")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *ppoCorpora != "" {
		if *corpora != "" {
			fmt.Fprintln(stderr, "policytrain: -ppo-corpus and -corpus are mutually exclusive")
			return 2
		}
		return runPPO(runPPOArgs{
			corpora: *ppoCorpora, init: *initCkpt, out: *out, statsOut: *statsOut, kinds: *ppoKinds,
			cfg: PPOConfig{Epochs: *epochs, Batch: *batch, LR: *lr, Seed: *seed, Holdout: *holdout, Clip: *clip,
				PPOClip: *ppoClip, PPOKL: *ppoKL, ValueWeight: *valueWeight, ValueHidden: *valueHidden,
				AdvNorm: *ppoAdvNorm, Baseline: *ppoBaseline, VDWM: *vdwm, VDWMMargin: *vdwmMargin, Log: stdout},
		}, stdout, stderr)
	}
	if *corpora == "" || *out == "" {
		fmt.Fprintln(stderr, "policytrain: -corpus and -out are required")
		fs.Usage()
		return 2
	}

	var examples []policynet.Example
	for _, path := range strings.Split(*corpora, ",") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		exs, stats, err := policynet.Load(path)
		if err != nil {
			fmt.Fprintf(stderr, "policytrain: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "corpus %s: %d records, %d skipped (no candidates), %d loaded (%d with outcome)\n",
			path, stats.Records, stats.Skipped, len(exs), stats.WithOutcome)
		examples = append(examples, exs...)
	}
	if len(examples) == 0 {
		fmt.Fprintln(stderr, "policytrain: no example loaded")
		return 1
	}

	mode, err := policynet.ParseLossMode(*lossMode)
	if err != nil {
		fmt.Fprintf(stderr, "policytrain: %v\n", err)
		return 2
	}
	kindModes, err := parseKindModes(*kindLoss)
	if err != nil {
		fmt.Fprintf(stderr, "policytrain: %v\n", err)
		return 2
	}

	cfg := Config{
		Epochs: *epochs, Batch: *batch, LR: *lr, Seed: *seed, Holdout: *holdout, HoldoutBy: *holdoutBy,
		Embed: *embed, Hidden: *hidden,
		Mode:       mode,
		RankWeight: *rankWeight, HuberDelta: *huberDelta,
		OverrideWeight: *overrideWeight,
		Clip:           *clip,
		ResidualInit:   *residualInit,
		KindModes:      kindModes,
		ValueHidden:    *valueHidden,
		ValueWeight:    *valueWeight,
		ValueBlend:     *valueBlend,
		Log:            stdout,
	}
	res, err := Train(examples, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "policytrain: %v\n", err)
		return 1
	}
	final := res.Epochs[len(res.Epochs)-1]
	if err := res.Model.SaveCheckpoint(*out); err != nil {
		fmt.Fprintf(stderr, "policytrain: %v\n", err)
		return 1
	}
	info, err := os.Stat(*out)
	size := int64(-1)
	if err == nil {
		size = info.Size()
	}
	fmt.Fprintf(stdout, "checkpoint %s: rows=%d h=%d hidden=%d value-hidden=%d (%d bytes), encoder hash %#016x\n",
		*out, res.Model.Rows, res.Model.H, res.Model.Hidden, res.Model.ValueHidden, size, policynet.EncoderHash())
	fmt.Fprintf(stdout, "holdout per-kind top-1 among labelled options (blended below is just that, blended; loss mode %s):\n", mode)
	fmt.Fprintf(stdout, "  (split by %s; m-ovr/m-keep are model top-1 on the teacher-override/kept subsets, ovr-n the override count, m-bot the fraction of model picks that are a bot pick)\n", holdoutUnit(*holdoutBy))
	fmt.Fprintf(stdout, "  %-10s %8s %8s %8s %8s %8s %8s %8s %8s %8s\n", "kind", "model", "bot", "first", "random", "n", "ovr-n", "m-ovr", "m-keep", "m-bot")
	for _, k := range res.ByKind {
		fmt.Fprintf(stdout, "  %-10s %8.3f %8.3f %8.3f %8.3f %8d %8d %8.3f %8.3f %8.3f\n",
			k.Kind, k.ModelTop1, k.BotTop1, k.FirstTop1, k.RandomTop1, k.Eligible,
			k.OverrideN, k.ModelOverrideTop1, k.ModelKeepTop1, k.ModelPicksBot)
	}
	if v := res.Value; v != nil {
		fmt.Fprintf(stdout, "value head holdout (split by %s; blend %g; %d train / %d holdout examples with a target):\n", holdoutUnit(*holdoutBy), *valueBlend, v.TrainN, v.HoldoutN)
		fmt.Fprintf(stdout, "  %-22s %10s %10s\n", "predictor", "log-loss", "brier")
		fmt.Fprintf(stdout, "  %-22s %10.6f %10.6f\n", "value head", v.LogLoss, v.Brier)
		fmt.Fprintf(stdout, "  %-22s %10.6f %10.6f\n", fmt.Sprintf("base rate %.4f", v.BaseRate), v.BaseLogLoss, v.BaseBrier)
	}
	fmt.Fprintf(stdout, "train %d examples, holdout %d, skipped %d; final train loss %.6f top1 %.3f (blended), holdout loss %.6f top1 %.3f (blended)\n",
		res.TrainN, res.HoldoutN, res.Skipped, final.TrainLoss, final.TrainTop1, final.HoldoutLoss, final.HoldoutTop1)
	return 0
}

// holdoutUnit names the split unit for the table header ("" is example).
func holdoutUnit(by string) string {
	if by == "" {
		return HoldoutByExample
	}
	return by
}

// parseKindModes parses a "kind=mode,kind=mode" list into the per-kind loss
// map. An empty list is nil, meaning every kind uses -loss.
func parseKindModes(spec string) (map[decision.Kind]policynet.LossMode, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, nil
	}
	out := map[decision.Kind]policynet.LossMode{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("bad kind-loss entry %q (want kind=mode)", part)
		}
		kind := strings.TrimSpace(kv[0])
		m, err := policynet.ParseLossMode(strings.TrimSpace(kv[1]))
		if err != nil {
			return nil, fmt.Errorf("kind-loss %s: %w", kind, err)
		}
		out[decision.Kind(kind)] = m
	}
	return out, nil
}

type runPPOArgs struct {
	corpora, init, out, statsOut, kinds string
	cfg                                 PPOConfig
}

// runPPO is the -ppo-corpus mode's CLI: load the corpora and the init
// checkpoint, run one PPO round, write the checkpoint and the readout.
func runPPO(a runPPOArgs, stdout, stderr io.Writer) int {
	if a.init == "" || a.out == "" {
		fmt.Fprintln(stderr, "policytrain: PPO mode needs -init and -out")
		return 2
	}
	if strings.TrimSpace(a.kinds) != "" {
		for _, k := range strings.Split(a.kinds, ",") {
			switch k = strings.TrimSpace(k); decision.Kind(k) {
			case decision.KAttackers, decision.KPriority, decision.KBlockers, decision.KTarget:
				a.cfg.Kinds = append(a.cfg.Kinds, decision.Kind(k))
			default:
				fmt.Fprintf(stderr, "policytrain: -ppo-kinds: %q is not a scored policynet kind (attackers, priority, blockers, target)\n", k)
				return 2
			}
		}
	}
	var examples []policynet.Example
	for _, path := range strings.Split(a.corpora, ",") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		exs, _, stats, err := policynet.LoadOnPolicy(path)
		if err != nil {
			fmt.Fprintf(stderr, "policytrain: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "on-policy corpus %s: %d records (%d with outcome, %d deviating from the bot)\n", path, stats.Records, stats.WithOutcome, stats.Deviated)
		examples = append(examples, exs...)
	}
	m, err := policynet.LoadCheckpointFile(a.init)
	if err != nil {
		fmt.Fprintf(stderr, "policytrain: -init %s: %v\n", a.init, err)
		return 1
	}
	res, err := TrainPPO(examples, m, a.cfg)
	if err != nil {
		fmt.Fprintf(stderr, "policytrain: %v\n", err)
		return 1
	}
	if err := res.Model.SaveCheckpoint(a.out); err != nil {
		fmt.Fprintf(stderr, "policytrain: %v\n", err)
		return 1
	}
	if a.statsOut != "" {
		if err := writePPOStats(a.statsOut, res); err != nil {
			fmt.Fprintf(stderr, "policytrain: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "checkpoint %s: residual %g value-hidden %d; train %d / holdout %d (policy %d), deviated %d, mean raw advantage %+.4f (%s baseline)\n",
		a.out, res.Model.ResidualW, res.Model.ValueHidden, res.TrainN, res.HoldoutN, res.PolicyN, res.Deviated, res.MeanAdv, res.Baseline)
	fmt.Fprintf(stdout, "  %-10s %6s %6s %7s %7s %8s %7s %8s %7s %7s %7s %8s %7s\n", "kind", "n", "dev", "dev%", "win", "adv", "p_old", "kl", "clip", "flip", "admitΔ", "disagree", "|adv|")
	for _, k := range res.ByKind {
		fmt.Fprintf(stdout, "  %-10s %6d %6d %7.2f %7.4f %+8.4f %7.4f %8.5f %7.4f %7.4f %7.4f %8.4f %7.4f\n",
			k.Kind, k.N, k.Deviated, k.DeviatedPct, k.WinRate, k.MeanAdv, k.MeanPOld, k.FinalKL, k.FinalClip, k.FinalFlip, k.FinalAdmits, k.MeanDisagree, k.MeanAbsAdv)
	}
	f := res.Final
	fmt.Fprintf(stdout, "final vs pi_old: kl %.6f clip-frac %.4f; value holdout n %d log loss %.6f (init %.6f) vs base rate %.4f log loss %.6f\n",
		f.KL, f.ClipFrac, f.ValueHoldoutN, f.ValueLogLoss, f.InitValueLogLoss, f.BaseRate, f.BaseLogLoss)
	for _, b := range f.ValueByTurn {
		fmt.Fprintf(stdout, "  value turns %-5s n %5d log loss %.4f vs bucket base rate %.4f log loss %.4f; AUC %.4f\n",
			b.Turns, b.N, b.LogLoss, b.BaseRate, b.BaseLogLoss, b.AUC)
	}
	return 0
}
