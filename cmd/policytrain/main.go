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
		embed          = fs.Int("embed", 128, "H, the shared embedding width")
		hidden         = fs.Int("hidden", 128, "hidden layer width")
		rankWeight     = fs.Float64("rank-weight", policynet.DefaultRankWeight, "weight of the ranking term against the value term (in CE mode the plain CE weight; in hybrid mode scaled by the per-example margin). INERT in pure CE mode: the per-batch gradient clip normalises any uniform loss scale away — tune -lr and -clip instead")
		huberDelta     = fs.Float64("huber-delta", policynet.DefaultHuberDelta, "Huber transition point (teacher values live in [0,1])")
		overrideWeight = fs.Float64("override-weight", 1, "multiplier on examples where the teacher overrode the bot (1 = no reweighting, >1 up-weights the informative decisions)")
		lossMode       = fs.String("loss", "ce", "loss mode: ce (pure argmax cross-entropy, the value term off), hybrid (value + margin-weighted rank) or value (value only)")
		clip           = fs.Float64("clip", 1, "global L2 gradient-norm cap per batch, on the summed batch gradient before the lr/batch scale (<= 0 disables; uncapped CE divergence is rejected with an error and no checkpoint)")
		residualInit   = fs.Float64("residual-init", 0, "fixed bot-prior residual weight: options in the bot's own answer score this much higher (0 disables; a positive value starts the model at the bot baseline)")
		kindLoss       = fs.String("kind-loss", "attackers=bce", "per-kind loss overrides as kind=mode,... (e.g. attackers=bce); kinds not listed keep -loss. The attackers default is bce: a per-option binary logistic loss trains the score LEVEL the seat's per-option admission rule reads, which argmax CE (shift-invariant) cannot")
	)
	if err := fs.Parse(args); err != nil {
		return 2
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
		Epochs: *epochs, Batch: *batch, LR: *lr, Seed: *seed, Holdout: *holdout,
		Embed: *embed, Hidden: *hidden,
		Mode:       mode,
		RankWeight: *rankWeight, HuberDelta: *huberDelta,
		OverrideWeight: *overrideWeight,
		Clip:           *clip,
		ResidualInit:   *residualInit,
		KindModes:      kindModes,
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
	fmt.Fprintf(stdout, "checkpoint %s: rows=%d h=%d hidden=%d (%d bytes), encoder hash %#016x\n",
		*out, res.Model.Rows, res.Model.H, res.Model.Hidden, size, policynet.EncoderHash())
	fmt.Fprintf(stdout, "holdout per-kind top-1 among labelled options (blended below is just that, blended; loss mode %s):\n", mode)
	fmt.Fprintf(stdout, "  %-10s %8s %8s %8s %8s %8s\n", "kind", "model", "bot", "first", "random", "n")
	for _, k := range res.ByKind {
		fmt.Fprintf(stdout, "  %-10s %8.3f %8.3f %8.3f %8.3f %8d\n",
			k.Kind, k.ModelTop1, k.BotTop1, k.FirstTop1, k.RandomTop1, k.Eligible)
	}
	fmt.Fprintf(stdout, "train %d examples, holdout %d, skipped %d; final train loss %.6f top1 %.3f (blended), holdout loss %.6f top1 %.3f (blended)\n",
		res.TrainN, res.HoldoutN, res.Skipped, final.TrainLoss, final.TrainTop1, final.HoldoutLoss, final.HoldoutTop1)
	return 0
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
