package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

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
		rankWeight     = fs.Float64("rank-weight", policynet.DefaultRankWeight, "weight of the margin-weighted ranking term against the value term")
		huberDelta     = fs.Float64("huber-delta", policynet.DefaultHuberDelta, "Huber transition point (teacher values live in [0,1])")
		overrideWeight = fs.Float64("override-weight", 1, "multiplier on examples where the teacher overrode the bot (1 = no reweighting, >1 up-weights the informative decisions)")
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
		fmt.Fprintf(stdout, "corpus %s: %d records, %d skipped (no candidates), %d loaded\n",
			path, stats.Records, stats.Skipped, len(exs))
		examples = append(examples, exs...)
	}
	if len(examples) == 0 {
		fmt.Fprintln(stderr, "policytrain: no example loaded")
		return 1
	}

	cfg := Config{
		Epochs: *epochs, Batch: *batch, LR: *lr, Seed: *seed, Holdout: *holdout,
		Embed: *embed, Hidden: *hidden,
		RankWeight: *rankWeight, HuberDelta: *huberDelta,
		OverrideWeight: *overrideWeight,
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
	fmt.Fprintf(stdout, "holdout per-kind top-1 among labelled options (blended below is just that, blended):\n")
	fmt.Fprintf(stdout, "  %-10s %8s %8s %8s %8s %8s\n", "kind", "model", "bot", "first", "random", "n")
	for _, k := range res.ByKind {
		fmt.Fprintf(stdout, "  %-10s %8.3f %8.3f %8.3f %8.3f %8d\n",
			k.Kind, k.ModelTop1, k.BotTop1, k.FirstTop1, k.RandomTop1, k.Eligible)
	}
	fmt.Fprintf(stdout, "train %d examples, holdout %d, skipped %d; final train loss %.6f top1 %.3f (blended), holdout loss %.6f top1 %.3f (blended)\n",
		res.TrainN, res.HoldoutN, res.Skipped, final.TrainLoss, final.TrainTop1, final.HoldoutLoss, final.HoldoutTop1)
	return 0
}
