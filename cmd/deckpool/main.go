// Command deckpool drafts seeded 8-seat pods with bot drafters, builds each
// seat's 40-card deck with a bot builder and writes every deck as a
// loadable deck file, plus an index (decks.jsonl), the seat pairs
// (pairs.tsv), a pool.tsv naming every deck and a manifest (manifest.json),
// under --out. Pod i opens with seed --seed+i and draws its pair and game
// seeds from PCG(--seed, salt<<32|i), so the same flags write byte-identical
// files; no game is played.
//
//	deckpool --set TRK --pods 125 --seed 1 --out DIR
package main

import (
	"os"

	"github.com/adams-shaun/gorge/cli"
)

// options is one deckpool invocation, gathered from the flags.
type options struct {
	set       string
	collation string
	pods      int
	seed      uint64
	drafter   string
	builder   string
	corpus    string
	out       string
	pairing   string
	pairsN    int
	gamesN    int
	split     string
	quiet     bool
}

func main() {
	os.Exit(cli.Execute(newRoot(), os.Args[1:], os.Environ(), os.Stdout, os.Stderr))
}

// newRoot is the command tree: one command, flags only.
// defaultBuilder is the builder deckpool uses without --builder: best23,
// the strongest measured (53.1% against balanced's 49.0%, 300 pods on the
// same pools).
const defaultBuilder = "best23"

func newRoot() *cli.Command {
	var o options
	return &cli.Command{
		Name:  "deckpool",
		Short: "draft seeded 8-seat pods and write every seat's deck as a deck file",
		Long: "deckpool opens pod i with seed --seed+i, drafts it with eight bots, builds each\n" +
			"seat's 40-card deck and writes it to OUT/decks/<set>-<seed>-s<seat>.json, with\n" +
			"OUT/decks.jsonl (one line per deck), OUT/pool.tsv (one row per deck, split\n" +
			"--split), OUT/pairs.tsv (the seat pairs --pairing names, one line per game, as\n" +
			"dplay's -pairs-file reads it) and OUT/manifest.json. --out must not exist or\n" +
			"be empty.",
		ValidArgs: cli.NoArgs,
		Flags: func(fs *cli.FlagSet) {
			fs.StringVar(&o.set, "set", "TRK", "set code the pods are cut from")
			fs.StringVar(&o.collation, "collation", "", "booster collation; empty = the set's default")
			fs.IntVar(&o.pods, "pods", 125, "draft pods to run (8 decks each); at least 1")
			fs.Uint64Var(&o.seed, "seed", 1, "first pod's seed; pod i uses seed+i")
			fs.StringVar(&o.drafter, "drafter", "balanced", "drafter: name, name@version or knobs:path.json (a pickbot.Tuning knob file)")
			fs.StringVar(&o.builder, "builder", defaultBuilder, "builder: name, name@version or knobs:path.json (a pickbot.Tuning knob file)")
			fs.StringVar(&o.corpus, "corpus", ".cards", "corpus directory (holds ir.gob.gz / cardsfolder)")
			fs.StringVar(&o.out, "out", "", "output directory (required); must not exist or be empty")
			fs.StringVar(&o.pairing, "pairing", "roundrobin",
				"how the seats pair for play: roundrobin (every pair once), random (-pairs-per-pod of them) or cross (seat i of pod p against seat i of pod p+1)")
			fs.IntVar(&o.pairsN, "pairs-per-pod", 8, "pairs per pod with --pairing random; at least 1")
			fs.IntVar(&o.gamesN, "games-per-pair", 1, "games each pair plays, each with its own seed; at least 1")
			fs.StringVar(&o.split, "split", "all", "pool.tsv split column; a player's -split reads the rows carrying it")
			fs.BoolVar(&o.quiet, "quiet", false, "print only the wrote line, not the pool summary")
		},
		Run: func(c *cli.Context, _ []string) error {
			if o.pods < 1 {
				return cli.Usagef("--pods must be >= 1, got %d", o.pods)
			}
			if o.out == "" {
				return cli.Usagef("--out is required")
			}
			return run(o, c.Stdout)
		},
	}
}
