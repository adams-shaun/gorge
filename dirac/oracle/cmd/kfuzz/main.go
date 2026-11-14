// Command kfuzz plays random generated-set pairings with gorge and a kernel
// in lockstep (dirac/oracle) and stops at the first divergence, writing its
// shrunk repro. Until the kernel lands, -kernel gorge (the default) is the
// gorge-vs-gorge self-check. -kernel <name> for a registered pool (oracle.Pools, built by
// dirac/oracle/pools/decks.py) plays that pool on the generated set of the
// same name; -list-pools prints the registry.
//
//	go run ./dirac/oracle/cmd/kfuzz -kernel fdn -pairs 200 -seed 1 -policy random
//	go run ./dirac/oracle/cmd/kfuzz -repro <repro.json>     # re-run a repro
//	go run ./dirac/oracle/cmd/kfuzz -replay <game.json>     # re-execute a kernel game in gorge
//	go run ./dirac/oracle/cmd/kfuzz -synth std-dsk -pairs 1500 # a synthetic evidence pool, dealt in memory
//
// On a registered pool it is an absolute instrument: every pair ends as clean,
// divergent, a stall, a hang (-game-timeout) or a classified skip, the verdict
// line counts each, and -absolute exits 1 on anything that is not clean, a
// capacity stall or a refusal. A hang fails every path. dirac/docs/verification.md
// has the exit table.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/state"
)

func main() { os.Exit(priorHungCode(run())) }

// priorHungEnv marks a continuation: runContinuation sets it just before the
// exec that hands the unplayed pairs to a fresh process image, so the image
// the run continues in can tell that an earlier segment hung.
const priorHungEnv = "GORGE_KFUZZ_PRIOR_HUNG"

// priorHung reports whether this image is a continuation of a segment that
// confirmed a hang.
func priorHung() bool { return os.Getenv(priorHungEnv) != "" }

// priorHungCode is the exit-code rule a continuation's image runs under: a
// hang fails its segment as always, and the continuation's code cannot rescue
// the run, so a clean segment of a continuation is still a failed run.
func priorHungCode(code int) int {
	if code == 0 && priorHung() {
		return 1
	}
	return code
}

// resolveKernel maps -kernel <registered pool other than fdn> to the fdn path
// on that pool's set: it forces -set <name> (an explicit different -set is an
// error) and defaults -decks to the pool's decks directory. Any other kernel
// comes back unchanged.
func resolveKernel(kernel, setName, decks string, explicit map[string]bool) (string, string, string, error) {
	if kernel == "fdn" {
		return kernel, setName, decks, nil
	}
	p, err := oracle.PoolByName(kernel)
	if err != nil {
		return kernel, setName, decks, nil
	}
	if explicit["set"] && setName != kernel {
		return "", "", "", fmt.Errorf("-kernel %s plays the %s set, not -set %q", kernel, kernel, setName)
	}
	if !explicit["decks"] {
		decks = oracle.PoolDecks(p.Name)
	}
	return "fdn", kernel, decks, nil
}

// addMiniDecks cuts every deck of pool to n cards (-cmd-mini; 0 leaves them):
// the pool's names then play the mini decks.
func addMiniDecks(c *oracle.Corpus, pool []oracle.PoolDeck, n int) error {
	if n <= 0 {
		return nil
	}
	names := make([]string, len(pool))
	for i, p := range pool {
		names[i] = p.Name
	}
	minis, err := oracle.MiniCommanderDecks(c, names, n)
	if err != nil {
		return err
	}
	c.AddDecks(minis)
	return nil
}

// runKernel is the side this run plays against, as a Repro records it.
var runKernel string

// kernelLabel names the side a run plays against: the synthetic pool, the
// card set the fdn path plays, or the kernel flag itself.
func kernelLabel(synth, kernel, setName string) string {
	switch {
	case synth != "":
		return "synth " + synth
	case kernel == "fdn":
		return "set " + setName
	}
	return kernel
}

// kernelFlags is the command line that plays against the side label names.
func kernelFlags(label string) string {
	if pool, ok := strings.CutPrefix(label, "synth "); ok {
		return "-synth " + pool
	}
	if set, ok := strings.CutPrefix(label, "set "); ok {
		return "-kernel fdn -set " + set
	}
	return "-kernel " + label
}

// reproRun checks rep, read from path, against this run: a repro replayed
// against another side, or on the other surface, is another game, so it fails
// rather than report "no divergence". The repro's surface replaces the default
// one; one given on the command line must agree with it. A repro that records
// neither (written before they were) runs as the command line says.
func reproRun(rep *oracle.Repro, path, surface string, surfaceGiven bool) (string, error) {
	if rep.Kernel != "" && rep.Kernel != runKernel {
		return "", fmt.Errorf("%s was played against %s, not %s: replay it with %s", path, rep.Kernel, runKernel, kernelFlags(rep.Kernel))
	}
	if rep.Surface != "" && rep.Surface != surface {
		if surfaceGiven {
			return "", fmt.Errorf("%s was played on the %s surface, not -surface %s", path, rep.Surface, surface)
		}
		surface = rep.Surface
	}
	return surface, nil
}

// newRepro is oracle.NewRepro with the side this run plays against.
func newRepro(m *oracle.Matchup, seed uint64, rc oracle.RunConfig, res *oracle.Result) *oracle.Repro {
	return stampRepro(oracle.NewRepro(m, seed, rc, res))
}

// stampRepro records the side this run plays against on r: the kernel label,
// and for a synth run the pool (a synth name is its glob pool name, which the
// lockstep tables match on). Other kernels leave Pool alone.
func stampRepro(r *oracle.Repro) *oracle.Repro {
	r.Kernel = runKernel
	if pool, ok := strings.CutPrefix(runKernel, "synth "); ok {
		r.Pool = pool
	}
	return r
}

func run() int {
	var (
		cardsDir   = flag.String("cards", "", "gorge corpus directory (default: $GORGE_CARDS, else .cards in or above the working directory)")
		decksDir   = flag.String("decks", "", "pool deck directory (default: the fdn pool's, under $GORGE_POOLS or the module's .pools, else its committed slice)")
		poolPath   = flag.String("pool", "", "pool.tsv (default: <decks>/../pool.tsv)")
		split      = flag.String("split", "all", "pool split the decks come from: train, eval or all")
		pairs      = flag.Int("pairs", 100, "deck pairs (one game each)")
		firstPair  = flag.Int("first-pair", 0, "index of the first pair")
		seed       = flag.Uint64("seed", 1, "base seed: pairs and game seeds derive from it (the pairing rule)")
		policy     = flag.String("policy", "random", "random, heuristic, explore (gorge's coverage-exploration bot, cmd/cardfuzz's) or first")
		kernel     = flag.String("kernel", "gorge", "the kernel under test: gorge (gorge-vs-gorge self-check), gorge-api (the same through the contract's Kernel surface), vanilla (package kernel on dirac/vanilla's decks), fdn (package kernel on a registered card set's pool pairings) or any other registered pool such as blb (fdn's path on that set and its pool, oracle.Pools; see -list-pools)")
		lead       = flag.String("lead", "gorge", "which side leads the lockstep: gorge or kernel (fdn/vanilla paths)")
		setName    = flag.String("set", "fdn", "the generated card set the fdn kernel plays (a name in dirac/cardset)")
		mulligans  = flag.Int("mulligans", 0, "London mulligans each player may take (0 skips the round)")
		elide      = flag.Bool("pass-elision", false, "elide priority decisions whose only option is pass (must match the kernel's)")
		maxTurns   = flag.Int("max-turns", 100, "turn cap; a capped game is a stall")
		maxDec     = flag.Int("max-decisions", 20000, "canonical decision cap; a capped game is a stall")
		workers    = flag.Int("workers", 1, "games in parallel")
		keepGoing  = flag.Bool("keep-going", false, "count every divergence instead of stopping at the first")
		shrink     = flag.Int("shrink", 200, "shrink reruns for the first divergence (0: none)")
		out        = flag.String("out", "", "directory for the first divergence's repro.json, and one file a bad game: capacity-<table>-pair<N>.repro.json for a capacity stall, unmodelled-<why>-pair<N>.repro.json for an unmodelled kernel stall, divergent-pair<N>.repro.json for a divergent game (default: none written)")
		reproPath  = flag.String("repro", "", "re-run this repro in lockstep and report")
		explain    = flag.String("explain", "", "print this repro's actions rendered, one a line, and exit (what a recorded repro's old readable field held)")
		replayPath = flag.String("replay", "", "re-execute this kernel game (koracle-game-2) in gorge and report")
		jsonOut    = flag.Bool("json", false, "print the summary as JSON")
		features   = flag.Bool("features", false, "vanilla/fdn: also assert the network inputs (dirac/feat vs policynet) at every decision")
		stats      = flag.Bool("stats", false, "fdn/blb: print each kernel pool's peak occupancy and the pair that reached it (kstats)")
		verbose    = flag.Bool("v", false, "list the unpriced plays by card (fdn: every game's first divergence)")
		surfaceArg = flag.String("surface", "", "the option surface both sides offer: autopay (neither side offers the plain casts gorge's planner never prices, oracle.Config.GorgeAutopaySurface) or full (default)")
		surface    = flag.Bool("autopay-surface", false, "deprecated alias of -surface autopay")
		absolute   = flag.Bool("absolute", false, "pool runs: exit 1 on any divergence, hang, panic, unmodelled stall, unknown option kind or error skip, or when no game was played; capacity stalls and refusals are counted and do not fail")
		gameLimit  = flag.Duration("game-timeout", 30*time.Second, "a game with no result after this long, twice (one retry), is recorded as a hang and the run moves on; a hang fails the run (0: no watchdog)")
		pairsFile  = flag.String("pairs-file", "", "pool runs: play these pairs instead of drawing from pool.tsv: deckA<TAB>deckB[<TAB>seed] per line (oracle.ExplicitPairs); -first-pair skips lines and -pairs, when given, caps them")
		evidence   = flag.String("evidence", "", "pool runs: play the cards the table does not refuse, unverified ones too, and write the cards the run's clean games evidence (oracle.Coverage.Evidence), one name a line, here")
		allowUnv   = flag.Bool("allow-unverified", false, "pool runs: play the cards the table does not refuse, unverified ones too, as -evidence does")
		implAll    = flag.Bool("implicates-all", false, "pool runs: name every implicated card instead of the top ten, and print one line per game that fails -absolute: bad-game, pair, seed, outcome, class, the implicated cards joined by |, the sources of its latest actions joined by |, the cards of both decks joined by |, tab-separated; a game that recorded an unknown option kind also prints unknown-game, pair, seed, the kinds joined by |")
		listPools  = flag.Bool("list-pools", false, "print the registered lockstep pools (name, inputs, label, decks, tree digest) and exit")
		listSynth  = flag.Bool("list-synth", false, "print the synthetic evidence pools, one a line, and exit")
		covPath    = flag.String("coverage", "", "write the run's per-card/per-ability coverage TSV (oracle.Coverage) here and print a coverage summary line")
		v2Mode     = flag.String("v2", "", "run a v2 engine-parity check instead of lockstep: \"transcript\" plays the same seeded games on dirac/cmd/sbv2dirac and gorge's env and diffs their transcripts")
		games      = flag.Int("games", 100, "-v2 transcript: seeded games per answerer")
		firstGame  = flag.Int("first-game", 0, "-v2 transcript: index of the first pool pair (repro anchor)")
		answerers  = flag.String("answerers", "first,rand", "-v2 transcript: comma-separated scripted answerers (first, rand)")
		v2Workers  = flag.Int("v2-workers", 4, "-v2 transcript: games in parallel")
		v2MaxDec   = flag.Int("v2-max-decisions", 500, "-v2 transcript: v2 decision cap per game (a prefix is compared; 0 uses 500)")
		v2NoChance = flag.Bool("v2-no-chance", false, "-v2 transcript: negative control: the gorge host keeps its secret-stream shuffles instead of dealing the dirac kernel's chance (must diff)")
		synth      = flag.String("synth", "", "play a synthetic evidence pool (make dirac-tables writes them): deal its decks in memory from -seed and report its reach; replaces -kernel, -set and -decks")
		synthDecks = flag.Int("synth-decks", oracle.SynthDecks, "-synth: decks to deal")
		seats      = flag.Int("seats", 2, "seats a game: 2 plays deck pairs; 3 or 4 plays pods of distinct decks (oracle.PodAt): -kernel gorge in any build, -kernel vanilla or a -synth pool in a build with -tags dirac_mp")
		pods       = flag.Int("pods", 0, "-seats above 2: pods to play (one game each; 0: -pairs)")
		cmdMini    = flag.Int("cmd-mini", 0, "-seats above 2: cut every commander deck of the pool to this many cards, its commanders and an even spread of the rest (oracle.MiniCommanderDeck), so a Commander pod fits the kernel's four-seat build: -kernel gorge, or a Commander pool such as cmd5 in a build with -tags dirac_mp")
		eventTail  = flag.Int("events", 0, "-repro: print the last N events of gorge's log when the game ends, objects named by card")
		refusedIn  = flag.Bool("play-refused", false, "pool runs: play the pairs holding a card refused by name too, as scripts/dirac-first-refusals.py settle does")
		focusPath  = flag.String("focus", "", "-synth: deal a focus pool instead: every deck holds -focus-per-deck cards of this list (one card a line, enablers after a tab, ';'-separated) beside the enablers they name; the run is named after the file (fx-dsk.txt: fx-dsk)")
		focusPer   = flag.Int("focus-per-deck", oracle.FocusPerDeck, "-focus: focus cards per deck")
		reachPath  = flag.String("reach", "", "-synth: write one row per card of the list here: card, how far it got, decks, games, plays in clean games")
		v2RandFull = flag.Bool("v2-rand-full", false, "-v2 transcript: no-op (a rand game is always compared whole; the flag keeps the old spelling working)")
		cpuProf    = flag.String("cpuprofile", "", "write a CPU profile here")
		memProf    = flag.String("memprofile", "", "write an allocation profile here and print the run's allocation totals")
	)
	flag.Parse()
	defer startProfile(*cpuProf, *memProf)()
	verboseFDN = *verbose
	reproEvents = *eventTail
	coveragePath = *covPath
	evidencePath = *evidence
	implicatesAll = *implAll
	statsOn = *stats
	if *listPools {
		for _, p := range oracle.Pools() {
			q, _ := oracle.PoolByName(p.Name)
			fmt.Printf("%s\t%s\t%s\t%d\t%s\n", p.Name, q.Inputs, p.Label, p.Decks, p.Tree)
		}
		return 0
	}
	if *listSynth {
		names, err := cardset.SynthNames()
		if err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 1
		}
		fmt.Println(strings.Join(names, "\n"))
		return 0
	}
	explicit := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	// -explain renders a recorded repro's actions and exits: no pools, no
	// cards, no game.
	if *explain != "" {
		rep, err := oracle.ReadRepro(*explain)
		if err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 1
		}
		for _, s := range rep.Explain() {
			fmt.Println(s)
		}
		return 0
	}
	var err error
	if *seats < 2 || *seats > oracle.MaxSeats {
		fmt.Fprintf(os.Stderr, "kfuzz: -seats %d: want 2 to %d\n", *seats, oracle.MaxSeats)
		return 2
	}
	if explicit["pods"] && *seats == 2 {
		fmt.Fprintln(os.Stderr, "kfuzz: -pods needs -seats above 2")
		return 2
	}
	if explicit["cmd-mini"] && (*seats == 2 || *cmdMini < 1 || *synth != "") {
		fmt.Fprintln(os.Stderr, "kfuzz: -cmd-mini needs -seats above 2, a positive card count and a pool, not -synth")
		return 2
	}
	// synthGorge plays a synthetic pool's decks gorge against gorge, the
	// harness check of a game of more than two seats.
	synthGorge := *synth != "" && *seats > 2 && explicit["kernel"] && *kernel == "gorge"
	if *seats > 2 {
		for _, name := range []string{"repro", "replay", "v2", "pairs-file", "coverage"} {
			if explicit[name] {
				fmt.Fprintf(os.Stderr, "kfuzz: -seats %d cannot be used with -%s\n", *seats, name)
				return 2
			}
		}
		if *pods > 0 {
			*pairs = *pods
		}
	}
	if synthGorge {
		delete(explicit, "kernel")
	}
	if *synth != "" {
		if err := checkSynthFlags(explicit); err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 2
		}
		*kernel = "fdn"
	} else if explicit["synth-decks"] || explicit["reach"] || explicit["focus"] || explicit["focus-per-deck"] {
		fmt.Fprintln(os.Stderr, "kfuzz: -synth-decks, -reach and -focus need -synth")
		return 2
	}
	*kernel, *setName, *decksDir, err = resolveKernel(*kernel, *setName, *decksDir, explicit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 2
	}
	if synthGorge {
		*kernel = "gorge"
	}
	if *seats > 2 && *kernel != "gorge" {
		podKernel := *kernel == "vanilla" || *kernel == "fdn" && (*synth != "" || *cmdMini > 0)
		switch {
		case !podKernel:
			fmt.Fprintf(os.Stderr, "kfuzz: -seats %d: a kernel game of more than two seats plays -kernel vanilla, a -synth pool or a -cmd-mini Commander pool; -kernel %s plays two seats (-kernel gorge plays any)\n", *seats, *kernel)
			return 2
		case *seats > dirac.MaxSeats:
			fmt.Fprintf(os.Stderr, "kfuzz: -seats %d: this dirac build holds %d seats; build with -tags dirac_mp for up to %d (-kernel gorge plays any)\n", *seats, dirac.MaxSeats, oracle.MaxSeats)
			return 2
		}
	}
	if *kernel != "fdn" {
		for _, name := range []string{"absolute", "pairs-file", "evidence"} {
			if explicit[name] {
				fmt.Fprintf(os.Stderr, "kfuzz: -%s needs -kernel fdn or a registered pool, not %q\n", name, *kernel)
				return 2
			}
		}
	}
	surfaceName, err := resolveSurface(*surfaceArg, *surface, explicit["autopay-surface"])
	if err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 2
	}
	runKernel = kernelLabel(*synth, *kernel, *setName)
	if *reproPath != "" {
		rep, err := oracle.ReadRepro(*reproPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 1
		}
		if surfaceName, err = reproRun(rep, *reproPath, surfaceName, explicit["surface"] || explicit["autopay-surface"]); err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 2
		}
	}
	if *decksDir == "" {
		*decksDir = oracle.PoolDecks("fdn")
	}
	sliced := false // the pool is a committed slice
	if *kernel == "fdn" && *synth == "" && *v2Mode == "" && !explicit["decks"] && !explicit["pool"] && !explicit["pairs-file"] && oracle.NeedDecks(*decksDir) != nil {
		if *seed != oracle.SliceSeed || *split != "all" || *firstPair+*pairs > oracle.SlicePairs {
			fmt.Fprintf(os.Stderr, "kfuzz: no %s pool on disk, and its committed slice holds only -split all -seed %d pairs 0 to %d: set GORGE_POOLS for others\n", *setName, oracle.SliceSeed, oracle.SlicePairs-1)
			return 1
		}
		dir, remove, err := oracle.ExtractSlice(*setName)
		if err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 1
		}
		defer remove()
		beforeExec = append(beforeExec, remove) // a continuation's exec skips the defer
		*decksDir, *poolPath, *pairsFile = filepath.Join(dir, "decks"), filepath.Join(dir, "pool.tsv"), filepath.Join(dir, "pairs.tsv")
		explicit["pairs"] = true
		sliced = true
	}
	if *lead != "gorge" && *lead != "kernel" {
		fmt.Fprintf(os.Stderr, "kfuzz: -lead %q: want gorge or kernel\n", *lead)
		return 2
	}
	if *lead == "kernel" && (*kernel == "gorge" || *kernel == "gorge-api") {
		fmt.Fprintln(os.Stderr, "kfuzz: -lead kernel needs -kernel fdn or vanilla (there is no kernel side for the gorge self-check)")
		return 2
	}
	if *cardsDir != "" {
		os.Setenv("GORGE_CARDS", *cardsDir) // the tables come from the corpus this run plays, not the one the cwd happens to sit under
	}
	var set cardset.Set
	switch {
	case *synth != "":
		// As -evidence: unverified cards play and decks are dealt without the refused
		// ones, unless -play-refused asks for the games a settling run needs.
		if set, err = cardset.SynthByName(*synth); err == nil && !*refusedIn {
			set = cardset.AllowUnverified(set)
		}
	case *refusedIn:
		set, err = cardset.RawByName(*setName)
	case *evidence != "" || *allowUnv:
		// Unverified cards play, or no card could earn evidence; a refused one stays out,
		// or its known divergence would fail the run.
		if set, err = cardset.ByName(*setName); err == nil {
			set = cardset.AllowUnverified(set)
		}
	default:
		set, err = cardset.ByName(*setName)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 2
	}
	if *cardsDir == "" {
		d, err := cardset.CorpusDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 2
		}
		*cardsDir = d
	}
	if *poolPath == "" {
		*poolPath = filepath.Join(filepath.Dir(*decksDir), "pool.tsv")
	}
	c, err := oracle.OpenCorpus(*cardsDir, *decksDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 1
	}
	if *v2Mode != "" {
		if *v2Mode != "transcript" {
			fmt.Fprintf(os.Stderr, "kfuzz: -v2 %q: want transcript\n", *v2Mode)
			return 2
		}
		if err := oracle.NeedDecks(*decksDir); err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 1
		}
		var ans []string
		for _, a := range strings.Split(*answerers, ",") {
			if a = strings.TrimSpace(a); a != "" {
				ans = append(ans, a)
			}
		}
		return runV2Transcript(v2TranscriptOpts{Corpus: c, PoolPath: *poolPath, Split: *split,
			DeckDir: *decksDir, FirstGame: *firstGame, Games: *games, Seed: *seed, Answerers: ans, Workers: *v2Workers, MaxDecisions: *v2MaxDec, Verbose: *verbose, NoChance: *v2NoChance, RandFull: *v2RandFull})
	}
	// -replay only needs the corpus, so route it before the kernel switch:
	// it applies to every side, not only gorge's.
	if *replayPath != "" {
		g, err := oracle.ReadKernelGame(*replayPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 1
		}
		r := oracle.Replay(c.Reg, g)
		fmt.Printf("replay: %d of %d actions applied, %d gorge intents, %d events\n", r.Applied, len(g.Actions), len(r.Log.Intents), len(r.Log.Events))
		if r.DivergedAt >= 0 {
			fmt.Printf("DIVERGED at sequence %d: %s: %s\n", r.DivergedAt, r.Reason, r.Detail)
			return 1
		}
		if r.Verified != nil {
			fmt.Printf("gorge log verification failed: %v\n", r.Verified)
			return 1
		}
		fmt.Println("replay OK (gorge log verified by engine/replay)")
		return 0
	}
	rc := oracle.RunConfig{Config: oracle.Config{PassElision: *elide, GorgeAutopaySurface: surfaceName == "autopay"}, MaxDecisions: *maxDec, MaxTurns: int32(*maxTurns)}
	var pol func(uint64) oracle.Policy
	switch *policy {
	case "random":
		pol = func(seed uint64) oracle.Policy {
			return oracle.AffordableTargetPolicy(oracle.RandomPolicy(seed))
		}
	case "heuristic":
		if *lead == "kernel" {
			fmt.Fprintln(os.Stderr, "kfuzz: -policy heuristic needs -lead gorge (the heuristic bot drives gorge's pending decision)")
			return 2
		}
		pol = oracle.HeuristicPolicy
	case "explore":
		if *lead == "kernel" {
			fmt.Fprintln(os.Stderr, "kfuzz: -policy explore needs -lead gorge (the explore bot drives gorge's pending decision)")
			return 2
		}
		pol = oracle.ExplorePolicy
	case "first":
		pol = func(uint64) oracle.Policy { return oracle.FirstPolicy() }
	default:
		fmt.Fprintf(os.Stderr, "kfuzz: -policy %q: want random, heuristic, explore or first\n", *policy)
		return 2
	}
	var mk oracle.SideFactory
	switch *kernel {
	case "gorge":
		mk = oracle.GorgeKernel(rc.Config)
	case "gorge-api":
		mk = oracle.GorgeKernelAPI(rc.Config)
	case "vanilla":
		if *reproPath != "" {
			return runRepro(c, *reproPath, *lead, rc, func(_ *oracle.Repro, rrc oracle.RunConfig) (oracle.SideFactory, error) {
				return oracle.RealKernel(vanilla.Table, nil, rrc.Config), nil
			})
		}
		return runVanilla(c, rc, *seats, *firstPair, *pairs, *seed, *mulligans, *gameLimit, *features, *out, *lead, pol)
	case "fdn":
		if *reproPath != "" {
			// before ReadPool: a repro carries its own deck names and needs
			// no -pool/pool.tsv.
			return runRepro(c, *reproPath, *lead, rc, func(rep *oracle.Repro, rrc oracle.RunConfig) (oracle.SideFactory, error) {
				// one table over every seat's deck: at two seats, set.Matchup's
				es := [][]cardset.Entry{oracle.ReproEntries(c.Reg, set, rep.Cards[0]), oracle.ReproEntries(c.Reg, set, rep.Cards[1])}
				for _, d := range rep.MoreCards {
					es = append(es, oracle.ReproEntries(c.Reg, set, d))
				}
				tbl, _, err := set.MatchupN(es)
				if err != nil {
					return nil, err
				}
				return oracle.SetSide(set, tbl, rrc.Config), nil
			})
		}
		var dealt []oracle.PoolDeck
		var reach reachFunc
		if *synth != "" {
			var err error
			if dealt, reach, err = dealSynth(c, set, *seed, *synthDecks, *reachPath, *focusPath, *focusPer); err != nil {
				fmt.Fprintln(os.Stderr, "kfuzz:", err)
				return 1
			}
		} else if err := oracle.NeedDecks(*decksDir); err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 1
		}
		o := sweepOpts{features: *features, lead: *lead, surface: surfaceName, workers: *workers, out: *out, timeout: *gameLimit, absolute: *absolute, reach: reach, mulligans: *mulligans, seats: *seats}
		if *synth == "" && *firstPair == 0 && *pairs == oracle.SynthPairs && *seed == oracle.EvidenceSeed {
			o.minGames = oracle.GateMinGames[*setName]
			if sliced {
				o.minGames = min(o.minGames, oracle.SliceMinGames)
			}
		}
		if *pairsFile != "" {
			ps, err := pairsFromFile(*pairsFile, *firstPair, *pairs, explicit["pairs"], *seed)
			if err != nil {
				fmt.Fprintln(os.Stderr, "kfuzz:", err)
				return 1
			}
			return runFDN(c, set, rc, ps, o, pol)
		}
		pool := dealt
		if *synth == "" {
			var err error
			if pool, err = oracle.ReadPool(*poolPath, *split); err != nil {
				fmt.Fprintln(os.Stderr, "kfuzz:", err)
				return 1
			}
		}
		if err := addMiniDecks(c, pool, *cmdMini); err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 1
		}
		ps := oracle.PodsFrom(pool, *firstPair, *pairs, *seed, *seats)
		if o.minGames > 0 {
			ps = oracle.PairingsPlayable(c, set, pool, *firstPair, *pairs, *seed)
		}
		return runFDN(c, set, rc, ps, o, pol)
	default:
		fmt.Fprintf(os.Stderr, "kfuzz: -kernel %q: want gorge or gorge-api (the real kernel's adapter lands with package kernel)\n", *kernel)
		return 2
	}
	if *reproPath != "" {
		rep, err := oracle.ReadRepro(*reproPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 1
		}
		m, err := rep.Matchup(c.Reg)
		if err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 1
		}
		res := rep.Rerun(m, mk, 200)
		if res.Div == nil {
			fmt.Printf("repro: no divergence in %d decisions\n", res.Decisions)
			return 0
		}
		printDiv(res.Div)
		return 1
	}
	if err := oracle.NeedDecks(*decksDir); err != nil && !synthGorge {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 1
	}
	var pool []oracle.PoolDeck
	if synthGorge {
		if pool, _, err = dealSynth(c, set, *seed, *synthDecks, "", *focusPath, *focusPer); err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 1
		}
	} else if pool, err = oracle.ReadPool(*poolPath, *split); err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 1
	}
	if err := addMiniDecks(c, pool, *cmdMini); err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 1
	}
	ps := oracle.PodsFrom(pool, *firstPair, *pairs, *seed, *seats)
	every := max(1, len(ps)/20)
	n := 0
	t0 := time.Now()
	sum := oracle.Fuzz(c, ps, mk, oracle.FuzzConfig{Run: rc, Mulligans: *mulligans, Policy: pol, StopAtFirst: !*keepGoing,
		Shrink: *shrink, Workers: *workers, Coverage: coveragePath != "", GameTimeout: *gameLimit, StopOnHang: *gameLimit > 0}, func(i int, p oracle.Pairing, r *oracle.Result) {
		n++
		if r.Div != nil {
			fmt.Fprintf(os.Stderr, "pair %d (%s, seed %d): %v\n", p.Index, strings.Join(p.Decks(), " vs "), p.Seed, r.Div)
		}
		if n%every == 0 {
			fmt.Fprintf(os.Stderr, "[%d/%d %.0fs]\n", n, len(ps), time.Since(t0).Seconds())
		}
	})
	if *jsonOut {
		b, _ := json.MarshalIndent(sum, "", " ")
		fmt.Println(string(b))
	} else {
		fmt.Printf("games %d  over %d  stalled %d  diverged %d  errors %d  decisions %d  lead %s  wall %.1fs  (%.1f games/s, %.0f decisions/s)\n",
			sum.Games, sum.Over, sum.Stalled, sum.Diverged, sum.Errors, sum.Decisions, *lead, sum.Wall.Seconds(),
			float64(sum.Games)/sum.Wall.Seconds(), float64(sum.Decisions)/sum.Wall.Seconds())
		st := sum.Stats
		fmt.Printf("gorge side: commits %d  elided %d  plan-casts %d  lowered %d  scripted %d  direct %d  internal %d  fallbacks %d  yields %d  aborts %d  refused %d  overflow(>%d) %d\n",
			st.Commits, st.Elided, st.PlanCasts, st.Lowered, st.Scripted, st.Direct, st.Internal, st.Fallbacks, st.Yields, st.Aborts, st.Refused, oracle.MaxOptions, st.Overflow)
		for _, kv := range []struct {
			name string
			m    map[string]int
		}{{"unknown", st.Unknown}, {"occurrence", st.Occurrence}, {"duplicate", st.Duplicate}, {"unpriced", st.Unpriced}, {"divergence classes", sum.ByClass}} {
			if len(kv.m) > 0 {
				fmt.Printf("%s: %s\n", kv.name, sortedCounts(kv.m))
			}
		}
	}
	if *verbose {
		printUnpriced(sum.Stats.UnpricedPlays)
	}
	if sum.Coverage != nil {
		if err := writeCoverage(coveragePath, sum.Coverage); err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 1
		}
	}
	for _, p := range sum.Hangs {
		fmt.Printf("pair %d (%s, seed %d): hang: no result after two watchdogs of %s; rerun with: %s\n", p.Index, vs(p), p.Seed, *gameLimit, rerunCmd(p.Index))
	}
	if sum.First != nil {
		printDiv(sum.First.Divergence)
		if *out != "" {
			if err := os.MkdirAll(*out, 0o755); err == nil {
				path := filepath.Join(*out, "repro.json")
				stampRepro(sum.First)
				if err := sum.First.Write(path); err == nil {
					fmt.Printf("repro (%d actions", len(sum.First.Actions))
					if sum.First.Shrunk != nil {
						fmt.Printf(", shrunk from %d in %d reruns", sum.First.Shrunk.From, sum.First.Shrunk.Attempts)
					}
					fmt.Printf("): %s\n", path)
				}
			}
		}
	}
	if sum.First != nil || sum.Hung > 0 {
		return failSegment(sum, ps)
	}
	return 0
}

// failSegment ends a failed generic segment: exit 1, after handing the pairs a
// hang left unplayed to a continuation (hangTail).
func failSegment(sum oracle.FuzzSummary, ps []oracle.Pairing) int {
	played := playedPrefix(sum, len(ps))
	if sum.Hung > 0 && played < len(ps) {
		return hangTail(played, ps[played].Index, len(ps)-played)
	}
	return 1
}

// playedPrefix is how many of a generic path's pairs this segment played: with
// StopOnHang the workers record the lowest pair index they received after the
// feed stopped (FuzzSummary.SkipAt), and the played pairs are exactly the
// prefix before it; with no stop every fed pair was played, and Games + Errors
// count each of them (a hang counts in Games).
func playedPrefix(sum oracle.FuzzSummary, n int) int {
	if sum.SkipAt < n {
		return sum.SkipAt
	}
	return sum.Games + sum.Errors
}

// hangTail ends a segment that confirmed a hang: the unplayed pairs rerun in a
// fresh process image (runContinuation execs, so the hung game's goroutine —
// which Go cannot cancel — dies with this image instead of holding a core for
// the rest of the run) and the segment fails. firstUnplayed/remaining name the
// unplayed pairs; playedN is how many pairs this segment played. A segment
// that played no pair (a worker can receive pair 0 only after the feed's stop)
// does not rerun: the continuation's arguments would be identical, so it could
// only repeat this segment, and the run fails with the pairs unplayed.
func hangTail(playedN, firstUnplayed, remaining int) int {
	if remaining <= 0 {
		return 1 // the hang was the last pair: nothing unplayed
	}
	if playedN == 0 {
		fmt.Fprintln(os.Stderr, "kfuzz: no continuation: the segment played no pair, a rerun would repeat it")
		return 1
	}
	runContinuation(firstUnplayed, remaining)
	return 1 // the exec replaced this image; this is the exec-failure path
}

// lockstepDir plays one game with lead leading: a fresh gorge side leads and
// mk's side follows, or mk's side leads and a fresh gorge side follows. It
// returns the mk side so the caller can close it.
func lockstepDir(m *oracle.Matchup, seed uint64, lead string, mk oracle.SideFactory, pol oracle.Policy, rc oracle.RunConfig) (oracle.Result, oracle.Side, error) {
	if lead == "kernel" {
		l, err := mk(m, seed)
		if err != nil {
			return oracle.Result{}, nil, err
		}
		f, err := oracle.NewGorgeSide(m, seed, rc.Config, nil)
		if err != nil {
			oracle.CloseSide(l)
			return oracle.Result{}, nil, err
		}
		return oracle.LockstepLed(m, seed, l, f, pol, rc), l, nil
	}
	k, err := mk(m, seed)
	if err != nil {
		return oracle.Result{}, nil, err
	}
	return oracle.Lockstep(m, seed, k, pol, rc), k, nil
}

// runRepro re-runs the repro at path under lead against the side newSide
// builds. rc carries the surface (the repro's own, reproRun); the repro supplies
// its own elision and turn cap. It does not call Repro.Rerun, which
// hardcodes gorge-led Lockstep and would ignore -lead kernel.
func runRepro(c *oracle.Corpus, path, lead string, rc oracle.RunConfig, newSide func(*oracle.Repro, oracle.RunConfig) (oracle.SideFactory, error)) int {
	rep, err := oracle.ReadRepro(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 1
	}
	m, err := rep.Matchup(c.Reg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 1
	}
	rrc := rc
	rrc.Config.PassElision = rep.PassElision
	rrc.MaxTurns = rep.MaxTurns
	rrc.MaxDecisions = len(rep.Actions) + 200
	if reproEvents > 0 {
		rrc.Finish = printEventTail(reproEvents)
	}
	mk, err := newSide(rep, rrc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 1
	}
	res, k, err := lockstepDir(m, rep.Seed, lead, mk, &oracle.ScriptPolicy{Actions: rep.Actions}, rrc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 1
	}
	oracle.CloseSide(k)
	if res.Div == nil {
		fmt.Printf("repro: no divergence in %d decisions\n", res.Decisions)
		return 0
	}
	printDiv(res.Div)
	return 1
}

// reproEvents is -events.
var reproEvents int

// printEventTail is a RunConfig.Finish that prints the last n events of gorge's log, each
// object by name and id.
func printEventTail(n int) func(*oracle.GorgeSide) {
	return func(g *oracle.GorgeSide) {
		name := func(id state.ObjID) string {
			if o := g.E.G.Obj(id); o != nil && o.Card != nil && len(o.Card.Faces) > 0 {
				return fmt.Sprintf("%s#%d", o.Card.Faces[0].Name, id)
			}
			return fmt.Sprintf("#%d", id)
		}
		for pl := state.PlayerID(0); int(pl) < len(g.E.G.Players); pl++ {
			for _, id := range g.E.G.Zone(state.ZHand, pl) {
				fmt.Printf("  hand p%d %s\n", pl, name(id))
			}
			for _, id := range g.E.G.Zone(state.ZBattlefield, pl) {
				o := g.E.G.Obj(id)
				att := ""
				if o.AttachedTo != 0 {
					att = " on " + name(o.AttachedTo)
				}
				pt := ""
				if g.E.IsCreature(id) {
					pt = fmt.Sprintf(" pt=%d/%d", g.E.Power(id), g.E.Toughness(id))
				}
				fmt.Printf("  bf p%d %s tapped=%v sick=%v atk=%v%s%s\n", pl, name(id), o.Tapped, o.SummonSick, o.IsAttacking, pt, att)
			}
		}
		evs := g.E.L.Events
		for _, e := range evs[max(0, len(evs)-n):] {
			line := fmt.Sprintf("  ev %d %s p%d", e.Seq, e.Kind, e.Player)
			if e.Obj != 0 {
				line += " obj=" + name(e.Obj)
			}
			if e.From != 0 || e.To != 0 {
				line += fmt.Sprintf(" zone %d>%d", e.From, e.To)
			}
			if e.Amount != 0 {
				line += fmt.Sprintf(" n=%d", e.Amount)
			}
			if e.Counter != "" {
				line += " ctr=" + e.Counter
			}
			if e.Text != "" {
				line += fmt.Sprintf(" %q", e.Text)
			}
			for _, id := range e.IDs {
				line += " " + name(id)
			}
			fmt.Println(line)
		}
	}
}

// coveragePath is -coverage and evidencePath -evidence: with both "" no coverage
// is collected.
var coveragePath, evidencePath string

// recentSourcesShown is how many of a game's latest sourced actions a bad-game line names.
const recentSourcesShown = 8

// implicatesAll is -implicates-all.
var implicatesAll bool

// printBadGame is -implicates-all's line for a game that fails -absolute, which
// scripts/dirac-first-refusals.py reads to choose what to refuse.
func printBadGame(c *oracle.Corpus, p oracle.Pairing, outcome, class string, implicated, recent []string) {
	if !implicatesAll {
		return
	}
	m, err := c.MatchupN(p.Decks(), 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return
	}
	fmt.Printf("bad-game\t%d\t%d\t%s\t%s\t%s\t%s\t%s\n", p.Index, p.Seed, outcome, class, strings.Join(implicated, "|"), strings.Join(recent, "|"), strings.Join(m.Table, "|"))
}

// writeEvidence writes the names cov's games evidence to path, one a line, and
// prints the count. A card in refused stays out whatever its games did: it is known
// to diverge somewhere.
func writeEvidence(path string, refused map[string]bool, cov *oracle.Coverage) error {
	names := slices.DeleteFunc(cov.Evidence(), func(n string) bool { return refused[n] })
	var b strings.Builder
	for _, n := range names {
		b.WriteString(n)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("evidence: %d of %d cards (at least %d plays in clean games, every own ability key used): %s\n", len(names), len(cov.Cards), oracle.EvidenceMinPlays, path)
	return nil
}

// writeCoverage writes cov's TSV to path and prints the summary line.
func writeCoverage(path string, cov *oracle.Coverage) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := cov.WriteTSV(f); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	t := cov.Totals()
	fmt.Printf("coverage: cards cast-or-played %d/%d, fully covered %d/%d, ability keys used %d/%d, policy fallbacks %d (of %d decisions in %d games)\n",
		t.Played, t.Cards, t.Full, t.Cards, t.KeysUsed, t.Keys, cov.Fallbacks, cov.Decisions, cov.Games)
	return nil
}

// runVanilla plays package kernel against gorge on dirac/vanilla's decks
// (every ordered pair of distinct decks, cycling) and reports the first
// divergence of each game by class.
//
// Game g (first <= g < first+games) is deck pair (g%n, ...) at seed seed+g,
// so `-first-pair g -pairs 1` re-runs game g of any run with the same -seed.
// Every divergent game prints one "DIV" line (game, decks, seed, class); with
// -out, the first three games of each class write <out>/<class>-g<g>.json
// (the lockstep's action prefix, a Repro).
func runVanilla(c *oracle.Corpus, rc oracle.RunConfig, seats, first, games int, seed uint64, mulligans int, timeout time.Duration, features bool, out string, lead string, pol func(uint64) oracle.Policy) int {
	mk := oracle.RealKernel(vanilla.Table, nil, rc.Config)
	var fs oracle.FeatStats
	if features {
		fp, err := oracle.FeatPool(c.Reg, vanilla.Table, nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 1
		}
		rc.Check = oracle.FeatCheck(fp, &fs)
	}
	n := len(vanilla.Decks)
	if seats > n {
		fmt.Fprintf(os.Stderr, "kfuzz: -seats %d: dirac/vanilla holds %d decks\n", seats, n)
		return 2
	}
	play := func(g int) played {
		ix := vanillaSeats(g, n, seats)
		labels := make([]string, len(ix))
		decks := make([][]kop.CardID, len(ix))
		for i, d := range ix {
			labels[i], decks[i] = fmt.Sprint("vanilla", d), vanilla.Decks[d]
		}
		s := seed + uint64(g)
		m, err := oracle.KernelMatchupN(c.Reg, vanilla.Table, labels, decks, nil, mulligans)
		if err != nil {
			return played{err: err}
		}
		res, k, err := lockstepDir(m, s, lead, mk, pol(s), rc)
		if err != nil {
			return played{err: err}
		}
		oracle.CloseSide(k)
		return played{m: m, res: res}
	}
	var feat *oracle.FeatStats
	if features {
		feat = &fs
	}
	return vanillaSweep(first, games, n, seats, seed, timeout, rc, out, lead, feat, play)
}

// vanillaSeats is game g's deck indexes of n decks, one per seat. Two seats
// play every ordered pair of distinct decks, cycling; more seats play n
// distinct decks from g's in a rotation.
func vanillaSeats(g, n, seats int) []int {
	a := g % n
	if seats <= 2 {
		return []int{a, (a + 1 + (g/n)%(n-1)) % n}
	}
	ix := make([]int, seats)
	for i := range ix {
		ix[i] = (a + i) % n
	}
	return ix
}

// vanillaSweep plays games first..first+games-1 serially, each under the watchdog
// (watchGame), and reports them. n is the deck count, for the game's deck indexes.
// A confirmed hang ends the segment: the report covers the games played, and the
// rest rerun in a fresh process image (hangTail -> runContinuation execs, so the
// hung game's goroutine dies with this image instead of holding a core until the
// run exits).
func vanillaSweep(first, games, n, seats int, seed uint64, timeout time.Duration, rc oracle.RunConfig, out, lead string, feat *oracle.FeatStats, play func(int) played) int {
	classes := map[string]int{}
	clean := 0
	written := map[string]int{}
	played := 0
	for g := first; g < first+games; g++ {
		ix := vanillaSeats(g, n, seats)
		vs, ds := fmt.Sprint(ix[0]), fmt.Sprint(ix[0])
		for _, d := range ix[1:] {
			vs, ds = fmt.Sprintf("%s vs %d", vs, d), fmt.Sprintf("%s,%d", ds, d)
		}
		s := seed + uint64(g)
		o := watchGame(g, timeout, !statsOn && feat == nil, play)
		played++
		if o.err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", o.err)
			return 1
		}
		if o.hang {
			classes["hang"]++
			fmt.Printf("game %d (vanilla %s, seed %d): hang: no result after two watchdogs of %s; rerun with: %s\n", g, vs, s, timeout, rerunCmd(g))
			break
		}
		m, res := o.m, o.res
		if res.Div == nil {
			clean++
			continue
		}
		cls := res.Div.What + " " + res.Div.Field
		classes[cls]++
		fmt.Printf("DIV game=%d decks=%s seed=%d decisions=%d class=%q\n", g, ds, s, res.Decisions, cls)
		if out != "" && written[cls] < 3 && m != nil {
			written[cls]++
			slug := strings.Map(func(r rune) rune {
				if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
					return r
				}
				return '_'
			}, cls)
			if err := os.MkdirAll(out, 0o755); err == nil {
				path := filepath.Join(out, fmt.Sprintf("%s-g%d.json", slug, g))
				if err := newRepro(m, s, rc, &res).Write(path); err != nil {
					fmt.Fprintln(os.Stderr, "kfuzz:", err)
				}
			}
		}
		if g < 12 || res.Div.What == "features" || verboseFDN {
			fmt.Printf("game %d (vanilla %s, seed %d): %v\n", g, vs, s, res.Div)
		}
		if verboseFDN {
			from := max(0, len(res.Actions)-5)
			for _, x := range res.Actions[from:] {
				fmt.Printf("    after %s\n", x.String())
			}
			printDiv(res.Div)
		}
	}
	fmt.Printf("vanilla: %d games, %d without divergence (lead %s); first divergences: %s\n", played, clean, lead, sortedCounts(classes))
	if feat != nil {
		fmt.Printf("features: %d decisions and %d options compared byte-for-byte\n", feat.Decisions, feat.Options)
	}
	for _, gap := range oracle.RealKernelGaps {
		fmt.Println("adapter gap:", gap)
	}
	if played < games {
		// The segment ended at the hang (an error path already returned): the
		// rest belong to a fresh process.
		return hangTail(played, first+played, games-played)
	}
	if len(classes) > 0 {
		return 1
	}
	return 0
}

// sweepOpts is one pool run's reporting and scheduling knobs.
type sweepOpts struct {
	features bool
	lead     string
	surface  string // "autopay" or "full"
	workers  int
	out      string
	timeout  time.Duration // -game-timeout; 0: no watchdog
	absolute bool
	reach    reachFunc       // -synth: reports the run's reach; nil elsewhere
	refused  map[string]bool // -evidence: the cards refused by name

	mulligans int // -mulligans: London mulligans each player may take; 0 skips the round

	minGames int // -absolute: fewer started games than this fail the run; 0: no floor

	seats int // -seats: 2 plays pairs; more plays pods (oracle.PodsFrom) and prints the pod line

	onGame func(*oracle.Result) // a test's look at each game, Actions included

	corpus *oracle.Corpus // where a bad-game line reads its decks
}

// played is one pair's outcome: a game with its Result, a skip (the pair was
// never played, and why), a hang (no result in time) or a setup failure.
type played struct {
	m    *oracle.Matchup
	res  oracle.Result
	cov  *oracle.Coverage
	skip error
	hang bool
	err  error
}

// The classes a skip counts under.
const (
	skipRefusedCard = "refused-card" // a deck holds a card the table refuses
	skipRefusedPair = "refused-pair" // the two decks are fine alone and the kernel refuses them together
	skipError       = "error"        // anything else: nobody decided this pair should not play
)

// skipClass classifies why a pair could not be built: cardset.RefusedError and
// cardset.InteractionRefusal by type, the older refusals by message, because those
// errors are untyped and live in dirac/cardset, whose sources key the card tables.
// Text it does not recognise is an error, so a reworded refusal fails loudly under
// -absolute instead of passing as a refusal.
// TestFuzzSkipReasonsCounted holds it to the real refusal functions.
func skipClass(err error) string {
	var refused *cardset.RefusedError
	if errors.As(err, &refused) {
		return skipRefusedCard
	}
	var inter *cardset.InteractionRefusal
	if errors.As(err, &inter) {
		return skipRefusedPair
	}
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "layer guard: "), // cardset.LayerRefusal
		strings.HasPrefix(msg, "Room name read: "),                // cardset.RoomNameRefusal
		strings.HasPrefix(msg, "ReduceCost Type Ability beside "): // cardset.AbReduceRefusal
		return skipRefusedPair
	}
	return skipError
}

// verdict is a pool run's counts. Games is every pair that started a game:
// Clean + Divergent + Unmodelled + Capacity + Hang. Skipped pairs are outside it.
type verdict struct {
	Games, Clean, Divergent, Unmodelled, Capacity, Hang int
	RefusedCard, RefusedPair, SkipError                 int
	Unknown                                             int // options with no canonical form

	TwoSeat int // of Capacity: twoseat stalls (dirac.StallTwoSeat), a two-seat table in a pod

	UnmodelledWhy, CapacityWhy map[string]int // stalls by reason
	RefusedPairWhy             map[string]int // refused pairs by reason (pairReason)
}

// pairReason is a refused pair's reason, without card names: the stall the game
// would have started with, or which older guard refused it.
func pairReason(err error) string {
	var inter *cardset.InteractionRefusal
	if errors.As(err, &inter) {
		return inter.Reason
	}
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "layer guard: "):
		return "layer"
	case strings.HasPrefix(msg, "Room name read: "):
		return "roomname"
	case strings.HasPrefix(msg, "ReduceCost Type Ability beside "):
		return "abreduce"
	}
	return "other"
}

// failures names what fails the run, in the verdict's words. Without -absolute
// only a divergence or a hang does, the historical meaning of exit 1.
func (v *verdict) failures(absolute bool) []string {
	var out []string
	add := func(n int, what string) {
		if n > 0 {
			out = append(out, fmt.Sprintf("%d %s", n, what))
		}
	}
	add(v.Divergent, "divergent")
	add(v.Hang, "hang")
	if absolute {
		add(v.Unmodelled, "unmodelled")
		add(v.Unknown, "unknown")
		add(v.SkipError, "error")
		if v.Games == 0 {
			out = append(out, "no game played")
		}
	}
	return out
}

// line is the verdict line.
func (v *verdict) line(name, lead, surface string) string {
	return fmt.Sprintf("%s: %d games (%d refused-card, %d refused-pair, %d error), %d clean, %d divergent, %d unmodelled [%s], %d capacity [%s], %d hang (lead %s, surface %s)",
		name, v.Games, v.RefusedCard, v.RefusedPair, v.SkipError, v.Clean, v.Divergent, v.Unmodelled, strings.TrimSpace(sortedCounts(v.UnmodelledWhy)), v.Capacity, strings.TrimSpace(sortedCounts(v.CapacityWhy)), v.Hang, lead, surface)
}

// twoSeatStall is the kernel stall name of dirac.StallTwoSeat.
const twoSeatStall = "twoseat"

// podLine is a pod run's census: twoseat stalls (a table that plays only two
// seats, a declared limit) apart from the other capacity stalls.
func (v *verdict) podLine(name string, seats int) string {
	return fmt.Sprintf("%s pods (%d seats): %d games, %d clean, %d divergent, %d capacity, %d twoseat, %d unmodelled, %d hang",
		name, seats, v.Games, v.Clean, v.Divergent, v.Capacity-v.TwoSeat, v.TwoSeat, v.Unmodelled, v.Hang)
}

// vs is a pairing's decks joined by " vs ".
func vs(p oracle.Pairing) string { return strings.Join(p.Decks(), " vs ") }

// legacyLine is the shorter verdict line. A script reads "N without divergence"
// from it and takes the first line that starts with the pool's name, so it prints
// first.
func (v *verdict) legacyLine(name, lead string, classes map[string]int) string {
	why := map[string]int{}
	for _, m := range []map[string]int{v.CapacityWhy, v.UnmodelledWhy} {
		for k, n := range m {
			why[k] += n
		}
	}
	return fmt.Sprintf("%s: %d games (%d skipped as unsupported), %d without divergence, %d kernel capacity stalls [%s] (lead %s); first divergences: %s",
		name, v.Games, v.RefusedCard+v.RefusedPair+v.SkipError, v.Clean, v.Capacity+v.Unmodelled, strings.TrimSpace(sortedCounts(why)), lead, sortedCounts(classes))
}

// runFDN plays package kernel (set's tables) against gorge on pool pairings and
// reports every pair by outcome.
// verboseFDN prints every game's first divergence in full (-v).
var verboseFDN bool

func runFDN(c *oracle.Corpus, set cardset.Set, rc oracle.RunConfig, ps []oracle.Pairing, o sweepOpts, pol func(uint64) oracle.Policy) int {
	o.corpus = c
	var fs oracle.FeatStats
	var ks kstats
	if statsOn {
		ks.install()
		defer ks.print(set.Name())
	}
	collect := coveragePath != "" || evidencePath != "" || o.reach != nil
	if evidencePath != "" {
		o.refused = map[string]bool{}
		for i, n := range set.Names()[:set.NumCards()] {
			if set.Status(i) == cardset.StatusRefused {
				o.refused[n] = true
			}
		}
	}
	play := func(i int) played {
		p := ps[i]
		ks.pair = p.Index
		m, tbl, err := oracle.SetMatchupN(set, c, p.Decks(), o.mulligans)
		if err != nil {
			return played{skip: err}
		}
		grc := rc
		if o.features {
			fp, err := oracle.FeatPool(c.Reg, tbl, nil)
			if err != nil {
				return played{err: err}
			}
			grc.Check = oracle.FeatCheck(fp, &fs)
		}
		var cov *oracle.Coverage
		if collect {
			grc = oracle.WithCoverage(grc, m, &cov)
		}
		gp := pol(p.Seed)
		res, k, err := lockstepDir(m, p.Seed, o.lead, oracle.SetSide(set, tbl, rc.Config), gp, grc)
		if err != nil {
			return played{skip: err}
		}
		if cov != nil {
			cov.Games, cov.Decisions, cov.Fallbacks = 1, res.Decisions, oracle.PolicyFallbacks(gp)
		}
		oracle.CloseSide(k)
		profDecisions.Add(int64(res.Decisions))
		if o.onGame != nil {
			o.onGame(&res)
		}
		if !verboseFDN && (o.out == "" || res.Div == nil && !strings.HasPrefix(res.Stall, oracle.KernelStallPrefix)) && (!implicatesAll || res.Clean()) {
			res.Actions = nil // only the reports of a bad game read them; don't hold every game's
		}
		return played{m: m, res: res, cov: cov}
	}
	// Pairs are independent, so they play on `workers` goroutines and are tallied
	// in pair order, which keeps the output identical to a serial run. -stats and
	// -features write shared counters per decision, so they keep one worker.
	workers := o.workers
	if statsOn || o.features || workers < 1 {
		workers = 1
	}
	out, playedN := playAll(len(ps), workers, o.timeout, !statsOn && !o.features, play)
	var feat *oracle.FeatStats
	if o.features {
		feat = &fs
	}
	name := set.Name()
	if focusName != "" {
		name = focusName
	}
	return finishSweep(name, ps, out, playedN, rc, o, feat)
}

// finishSweep reports the played prefix and reruns what a hang left unplayed.
// playAll stops feeding at the first confirmed hang and lets the in-flight pairs
// finish, so the played pairs are exactly ps[:playedN]; the segment's report
// covers those, and the rest play in a fresh process image (hangTail ->
// runContinuation execs, so the hung game's goroutine dies with this image
// instead of holding a core for the rest of the run). The continuation's code
// cannot rescue the segment: a hang fails it as always (priorHungCode).
func finishSweep(name string, ps []oracle.Pairing, out []played, playedN int, rc oracle.RunConfig, o sweepOpts, feat *oracle.FeatStats) int {
	if playedN < len(ps) {
		// The floor belongs to the whole form, and the segments' union covers it.
		o.minGames = 0
	}
	code := report(name, ps[:playedN], out[:playedN], rc, o, feat)
	if playedN < len(ps) {
		return hangTail(playedN, ps[playedN].Index, len(ps)-playedN)
	}
	return code
}

// playAll runs play(0..n-1) on workers goroutines under the per-game watchdog and
// returns the outcomes by index and how many pairs were played: feeding stops at
// the first confirmed hang, the in-flight pairs finish, a job a worker receives
// after that is not played (the continuation plays it), and the played pairs are
// then exactly the prefix [0, playedN) — the rest are zero, never played. retry
// decides whether a timed-out game is played once more (see watchGame).
func playAll(n, workers int, timeout time.Duration, retry bool, play func(i int) played) ([]played, int) {
	out := make([]played, n)
	var stop atomic.Bool
	skip := int64(n) // nothing skipped: where the played prefix ends, the first job received past the stop
	var mu sync.Mutex
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if stop.Load() {
					mu.Lock()
					if int64(i) < skip {
						skip = int64(i)
					}
					mu.Unlock()
					continue // received after a hang stopped the feed: the continuation plays it
				}
				out[i] = watchGame(i, timeout, retry, play)
				if out[i].hang {
					stop.Store(true)
				}
			}
		}()
	}
	fed := 0
	for i := 0; i < n; i++ {
		if stop.Load() {
			break
		}
		fed = i + 1
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if skip < int64(n) {
		return out, int(skip)
	}
	return out, fed
}

// watchGame runs play(i) on its own goroutine and waits at most timeout for it,
// twice when retry is set (oracle.WithinTimeoutRetry: one retry absorbs a host
// stall). A game past its time on both attempts is a hang: the goroutine cannot
// be stopped, so it leaks, but it only ever touches its own game and its
// buffered result channel, and the sweep stops feeding new pairs — the unplayed
// rest rerun in a fresh process image (runContinuation execs, which destroys
// this image's goroutines with it), so the leak holds a core until the segment
// ends, not for the run. A -stats or -features run keeps one attempt, because a stalled
// attempt shares that run's report-only counters and would double-count once its
// retry finishes. A panic outside the lockstep's own recover (building the game)
// is a divergence: one bad pair does not end the sweep.
func watchGame(i int, timeout time.Duration, retry bool, play func(int) played) played {
	wrapped := func() (p played) {
		defer func() {
			if r := recover(); r != nil {
				p = played{res: oracle.Result{Winner: -1, Div: &oracle.Divergence{What: "kfuzz", Field: "panic", Detail: fmt.Sprintf("%v\n%s", r, debug.Stack())}}}
			}
		}()
		return play(i)
	}
	return oracle.WithinTimeoutRetry(timeout, retry, wrapped, played{hang: true})
}

// kfuzzProg names this program the way a rerun command can execute it:
// os.Args[0], or `go run ./dirac/oracle/cmd/kfuzz` when the binary is a
// soon-deleted go-build temp (what a `go run` invocation leaves).
func kfuzzProg() string {
	if strings.Contains(os.Args[0], "go-build") {
		return "go run ./dirac/oracle/cmd/kfuzz"
	}
	return os.Args[0]
}

// rerunCmd is the command that replays one pair in a fresh kfuzz: the pair's
// index and one pair, so it reruns exactly the game the line reports.
func rerunCmd(pair int) string {
	words := []string{kfuzzProg()}
	for _, a := range contArgs(pair, 1) {
		if a == "" || strings.ContainsAny(a, " \t\n\"'\\$`*?[]{}()<>|&;!#~") {
			a = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'" // pastes back as one word
		}
		words = append(words, a)
	}
	return strings.Join(words, " ")
}

// contArgs is os.Args[1:] with every -pods, -first-pair and -pairs occurrence
// dropped and -first-pair <firstPair> -pairs <pairs> appended, so a continuation
// reruns exactly the pairs its caller leaves unplayed, whatever the run drew
// them from: pool, pod, synth, -pairs-file (whose Index is the line index) and
// vanilla (whose game index is the pair index) all make pair p a pure function
// of (seed, p), and last occurrence wins in flag.Parse, so -flag v, -flag=v and
// the --flag spellings all drop. -pods drops because a pod run's count is -pairs
// (oracle.PodAt makes pod p of (seed, p)); the appended -pairs names it.
func contArgs(firstPair, pairs int) []string {
	drop := map[string]bool{"pods": true, "first-pair": true, "pairs": true}
	args := os.Args[1:]
	out := make([]string, 0, len(args)+4)
	for i := 0; i < len(args); i++ {
		if name, _, hasVal := strings.Cut(args[i], "="); drop[strings.TrimLeft(name, "-")] {
			if !hasVal && i+1 < len(args) {
				i++ // -pods 3: the value is the next word, as flag.Parse reads it
			}
			continue
		}
		out = append(out, args[i])
	}
	return append(out, "-first-pair", strconv.Itoa(firstPair), "-pairs", strconv.Itoa(pairs))
}

// beforeExec holds the cleanups of run's deferred resources that runContinuation
// must run itself, since syscall.Exec never unwinds the stack.
var beforeExec []func()

// runContinuation hands the pairs a hang left unplayed to a fresh process
// image: it execs this program with contArgs(firstPair, pairs) — same pid, new
// image — so the hung game's goroutine, which Go cannot cancel, is destroyed
// with the segment's image instead of spinning a core until the run exits. It
// sets priorHungEnv first, so the new image fails even a clean segment (a hang
// already failed the run; priorHungCode), and it never returns: the exec'd
// image's exit code is the run's. The segments' file flags (-coverage,
// -evidence, -reach, -out/repro.json) are inherited unchanged, so the last
// segment's file wins — acceptable, because a hang already fails the run, and
// the per-pair files under -out are pair-numbered and survive; a
// -cpuprofile/-memprofile is not flushed past the exec (its stop is deferred in
// run). An exec error prints and returns 1, leaving the pairs unplayed. Tests
// stub this var.
var runContinuation = func(firstPair, pairs int) int {
	os.Setenv(priorHungEnv, "1")
	for _, f := range beforeExec {
		f() // the exec skips run's defers: drop the extracted slice dir now
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "kfuzz: continuation: %v\n", err)
		return 1
	}
	argv := append([]string{os.Args[0]}, contArgs(firstPair, pairs)...)
	if err := syscall.Exec(exe, argv, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "kfuzz: continuation: %v\n", err)
		return 1
	}
	return 1 // unreachable: the exec replaced this image
}

// writePairRepro writes one bad game's repro as dir/name, so the game replays from
// its own cards after a later land re-deals the pool. No dir (no -out) or no
// matchup writes nothing; a failed write is reported and the sweep goes on.
func writePairRepro(dir, name string, m *oracle.Matchup, seed uint64, rc oracle.RunConfig, res *oracle.Result) {
	if dir == "" || m == nil {
		return
	}
	err := os.MkdirAll(dir, 0o755)
	if err == nil {
		err = newRepro(m, seed, rc, res).Write(filepath.Join(dir, name))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
	}
}

// report tallies the outcomes in pair order, prints the verdict and returns the
// exit code.
func report(name string, ps []oracle.Pairing, out []played, rc oracle.RunConfig, o sweepOpts, feat *oracle.FeatStats) int {
	classes := map[string]int{}
	skips := map[string]int{}
	implicated := map[string]int{}
	v := verdict{CapacityWhy: map[string]int{}, UnmodelledWhy: map[string]int{}, RefusedPairWhy: map[string]int{}}
	var st oracle.Stats
	var cov *oracle.Coverage
	if coveragePath != "" || evidencePath != "" || o.reach != nil {
		cov = oracle.NewCoverage()
	}
	first := -1 // index in ps of the first divergence, pair order
	for i, p := range ps {
		if out[i].err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", out[i].err)
			return 1
		}
		if out[i].hang {
			v.Games++
			v.Hang++
			classes["hang"]++
			fmt.Printf("pair %d (%s, seed %d): hang: no result after two watchdogs of %s; rerun with: %s\n", p.Index, vs(p), p.Seed, o.timeout, rerunCmd(p.Index))
			printBadGame(o.corpus, p, "hang", "hang", nil, nil)
			continue
		}
		if err := out[i].skip; err != nil {
			cls := skipClass(err)
			switch cls {
			case skipRefusedCard:
				v.RefusedCard++
			case skipRefusedPair:
				v.RefusedPair++
				v.RefusedPairWhy[pairReason(err)]++
			default:
				v.SkipError++
			}
			skips[cls+": "+err.Error()]++
			if cls == skipError || verboseFDN {
				fmt.Printf("pair %d (%s, seed %d): %s: %v\n", p.Index, vs(p), p.Seed, cls, err)
			}
			continue
		}
		m, res := out[i].m, out[i].res
		if cov != nil {
			cov.MergeClean(out[i].cov, &res)
		}
		st.Add(res.Stats)
		v.Games++
		if implicatesAll && len(res.Stats.Unknown) > 0 {
			kinds := make([]string, 0, len(res.Stats.Unknown))
			for k := range res.Stats.Unknown {
				kinds = append(kinds, k)
			}
			slices.Sort(kinds)
			fmt.Printf("unknown-game\t%d\t%d\t%s\n", p.Index, p.Seed, strings.Join(kinds, "|"))
		}
		if res.Stall == oracle.GorgeTriggerCapStall {
			v.Capacity++
			v.CapacityWhy[res.Stall]++
			if verboseFDN || o.absolute {
				fmt.Printf("pair %d (%s, seed %d): %s stall after %d decisions\n", p.Index, vs(p), p.Seed, res.Stall, res.Decisions)
			}
			continue
		}
		if res.Div == nil && strings.HasPrefix(res.Stall, oracle.KernelStallPrefix) {
			why := strings.TrimPrefix(res.Stall, oracle.KernelStallPrefix)
			if res.StallUnmodelled {
				v.Unmodelled++
				v.UnmodelledWhy[why]++
				if verboseFDN || o.absolute {
					fmt.Printf("pair %d (%s, seed %d): %s unmodelled stall after %d decisions\n", p.Index, vs(p), p.Seed, res.Stall, res.Decisions)
				}
				printBadGame(o.corpus, p, "unmodelled", why, nil, res.RecentSources(m, recentSourcesShown))
				writePairRepro(o.out, fmt.Sprintf("unmodelled-%s-pair%d.repro.json", why, p.Index), m, p.Seed, rc, &res)
				continue
			}
			v.Capacity++
			v.CapacityWhy[why]++
			if why == twoSeatStall {
				v.TwoSeat++
			}
			if verboseFDN {
				fmt.Printf("pair %d (%s, seed %d): %s capacity stall after %d decisions; latest sources: %s\n", p.Index, vs(p), p.Seed, res.Stall, res.Decisions, strings.Join(res.RecentSources(m, 40), "|"))
			}
			// the game up to the stall, one file a pair: a test that plays it on
			// fails for as long as the table stalls there
			writePairRepro(o.out, fmt.Sprintf("capacity-%s-pair%d.repro.json", why, p.Index), m, p.Seed, rc, &res)
			continue
		}
		if res.Div == nil {
			v.Clean++
			continue
		}
		v.Divergent++
		cls := res.Div.What + " " + res.Div.Field
		if res.Div.What == "kernel" {
			cls = "kernel " + firstLineOf(res.Div.Detail)
		}
		classes[cls]++
		if first < 0 {
			first = i
		}
		// before -v's annotation, which rewrites the option and field text
		implicates := res.Div.Implicates(m)
		for _, n := range implicates {
			implicated[n]++
		}
		printBadGame(o.corpus, p, "divergent", cls, implicates, res.RecentSources(m, recentSourcesShown))
		writePairRepro(o.out, fmt.Sprintf("divergent-pair%d.repro.json", p.Index), m, p.Seed, rc, &res)
		if res.Div.What == "features" {
			fmt.Printf("pair %d (%s): %s\n", p.Index, vs(p), res.Div.Detail)
		}
		if verboseFDN {
			printVerboseDiv(p, m, &res, implicates)
		}
	}
	for _, n := range st.Unknown {
		v.Unknown += n
	}
	fmt.Println(v.legacyLine(name, o.lead, classes))
	fmt.Println(v.line(name, o.lead, o.surface))
	if o.seats > 2 {
		fmt.Println(v.podLine(name, o.seats))
	}
	if b, err := json.Marshal(classes); err == nil {
		fmt.Printf("%s-classes-json: %s\n", name, b)
	}
	if len(v.RefusedPairWhy) > 0 {
		fmt.Printf("refused-pair: %s\n", strings.TrimSpace(sortedCounts(v.RefusedPairWhy)))
	}
	for _, l := range skipLines(skips) {
		fmt.Println(l)
	}
	if len(implicated) > 0 {
		shown := 10
		if implicatesAll {
			shown = len(implicated)
		}
		fmt.Println("implicates:", topCounts(implicated, shown))
	}
	for _, kv := range []struct {
		name string
		m    map[string]int
	}{{"unknown", st.Unknown}, {"unpriced", st.Unpriced}} {
		if len(kv.m) > 0 {
			fmt.Printf("%s: %s\n", kv.name, sortedCounts(kv.m))
		}
	}
	if verboseFDN {
		printUnpriced(st.UnpricedPlays)
	}
	if feat != nil {
		fmt.Printf("features: %d decisions and %d options compared byte-for-byte\n", feat.Decisions, feat.Options)
	}
	if coveragePath != "" {
		if err := writeCoverage(coveragePath, cov); err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 1
		}
	}
	if evidencePath != "" {
		if err := writeEvidence(evidencePath, o.refused, cov); err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 1
		}
	}
	if o.reach != nil {
		if err := o.reach(ps, out, cov); err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 1
		}
	}
	if o.out != "" && first >= 0 && out[first].m != nil {
		if err := os.MkdirAll(o.out, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
		} else {
			path := filepath.Join(o.out, "repro.json")
			if err := newRepro(out[first].m, ps[first].Seed, rc, &out[first].res).Write(path); err != nil {
				fmt.Fprintln(os.Stderr, "kfuzz:", err)
			} else {
				fmt.Printf("repro (%d actions): %s\n", len(out[first].res.Actions), path)
			}
		}
	}
	fails := v.failures(o.absolute)
	if o.absolute {
		if v.Games < o.minGames {
			fails = append(fails, fmt.Sprintf("%d games started, under the pool's floor of %d (oracle.GateMinGames)", v.Games, o.minGames))
		}
		if len(fails) > 0 {
			fmt.Println("absolute: FAIL:", strings.Join(fails, ", "))
		} else {
			fmt.Println("absolute: OK")
		}
	}
	if len(fails) > 0 {
		return 1
	}
	return 0
}

// printVerboseDiv is -v's report of one divergent game: the last actions and the
// divergence, with every object annotated with its card.
func printVerboseDiv(p oracle.Pairing, m *oracle.Matchup, res *oracle.Result, implicates []string) {
	fmt.Printf("pair %d (%s, seed %d) after %d decisions:\n", p.Index, vs(p), p.Seed, res.Decisions)
	if m == nil {
		printDiv(res.Div)
		return
	}
	name := func(o int) string {
		if n := m.InstanceCard(o); n != "" {
			return n
		}
		return "token"
	}
	obj := regexp.MustCompile(`(#|Cards\[)(\d+)`)
	// a stack entry's source and an attachment name an object by its bare index
	byIndex := regexp.MustCompile(`^(Cards\[\d+\]\.AttachedTo|Stack\[\d+\]\.Source): ref=(\d+) kernel=(\d+)$`)
	ann := func(x string) string {
		if sm := byIndex.FindStringSubmatch(x); sm != nil {
			r, _ := strconv.Atoi(sm[2])
			k, _ := strconv.Atoi(sm[3])
			return fmt.Sprintf("%s: ref=%d{%s} kernel=%d{%s}", sm[1], r, name(r), k, name(k))
		}
		return obj.ReplaceAllStringFunc(x, func(t string) string {
			sm := obj.FindStringSubmatch(t)
			n, _ := strconv.Atoi(sm[2])
			return t + "{" + name(n) + "}"
		})
	}
	from := max(0, len(res.Actions)-5)
	for _, a := range res.Actions[from:] {
		fmt.Printf("    after %s\n", ann(a.String()))
	}
	d := *res.Div
	for i := range d.Diff {
		d.Diff[i] = ann(d.Diff[i])
	}
	for i := range d.RefOptions {
		d.RefOptions[i] = ann(d.RefOptions[i])
	}
	for i := range d.KerOptions {
		d.KerOptions[i] = ann(d.KerOptions[i])
	}
	printDiv(&d)
	if len(implicates) > 0 {
		fmt.Println("  implicates:", strings.Join(implicates, ", "))
	}
}

// skipLines prints the skips by class and reason, most frequent first, the first 10.
func skipLines(skips map[string]int) []string {
	ks := sortedKeysByCount(skips)
	var out []string
	for i, k := range ks {
		if i == 10 {
			out = append(out, fmt.Sprintf("skips: ... %d more reasons", len(ks)-i))
			break
		}
		out = append(out, fmt.Sprintf("skips: %d %s", skips[k], k))
	}
	return out
}

// topCounts prints m's n largest counts as "name=count, name=count". Names are
// card names, so the separator is not a space.
func topCounts(m map[string]int, n int) string {
	ks := sortedKeysByCount(m)
	if len(ks) > n {
		ks = ks[:n]
	}
	parts := make([]string, len(ks))
	for i, k := range ks {
		parts[i] = fmt.Sprintf("%s=%d", k, m[k])
	}
	return strings.Join(parts, ", ")
}

func sortedKeysByCount(m map[string]int) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return m[ks[i]] > m[ks[j]] || m[ks[i]] == m[ks[j]] && ks[i] < ks[j] })
	return ks
}

// resolveSurface reads -surface and its alias -autopay-surface.
func resolveSurface(name string, alias, aliasSet bool) (string, error) {
	switch name {
	case "":
		if alias {
			return "autopay", nil
		}
		return "full", nil
	case "autopay", "full":
		if aliasSet && alias != (name == "autopay") {
			return "", fmt.Errorf("-surface %s contradicts -autopay-surface=%v", name, alias)
		}
		return name, nil
	}
	return "", fmt.Errorf("-surface %q: want autopay or full", name)
}

// pairsFromFile is the pair list of a -pairs-file: oracle.ExplicitPairs' lines
// from line first, at most n when limit is set, with oracle.PairSeed's seeds. A
// pair's Index is its line among the pairs, so -first-pair K -pairs 1 re-runs the
// pair a sweep reported as K.
func pairsFromFile(path string, first, n int, limit bool, seed uint64) ([]oracle.Pairing, error) {
	lines, err := oracle.ExplicitPairs(path)
	if err != nil {
		return nil, err
	}
	var out []oracle.Pairing
	for i, l := range lines {
		if i < first {
			continue
		}
		if limit && len(out) == n {
			break
		}
		s, err := oracle.PairSeed(l, i, seed)
		if err != nil {
			return nil, fmt.Errorf("%s pair %d: %w", path, i, err)
		}
		out = append(out, oracle.Pairing{Index: i, A: l.A, B: l.B, Seed: s})
	}
	return out, nil
}

// printUnpriced lists the unpriced plays by planner detail and label, most
// frequent first (the first 40).
func printUnpriced(m map[string]int) {
	if len(m) == 0 {
		return
	}
	fmt.Println("unpriced plays (legal plays the autopay vocabulary leaves out):")
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return m[ks[i]] > m[ks[j]] || m[ks[i]] == m[ks[j]] && ks[i] < ks[j] })
	for i, k := range ks {
		if i >= 40 {
			break
		}
		fmt.Printf("  %6d  %s\n", m[k], k)
	}
}

func firstLineOf(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}

func printDiv(d *oracle.Divergence) {
	fmt.Println(d.String())
	// A position shift is usually the echo of a card that moved, so the lines naming
	// something else come first.
	diff := make([]string, 0, len(d.Diff))
	for _, l := range d.Diff {
		if !strings.HasSuffix(strings.SplitN(l, ": ", 2)[0], ".Pos") {
			diff = append(diff, l)
		}
	}
	for _, l := range d.Diff {
		if strings.HasSuffix(strings.SplitN(l, ": ", 2)[0], ".Pos") {
			diff = append(diff, l)
		}
	}
	for i, l := range diff {
		if i >= 20 {
			fmt.Printf("  ... %d more\n", len(diff)-i)
			break
		}
		fmt.Println("  " + l)
	}
	if len(d.RefOptions)+len(d.KerOptions) > 0 {
		fmt.Printf("  gorge  options: %v\n  kernel options: %v\n", d.RefOptions, d.KerOptions)
	}
}

func sortedCounts(m map[string]int) string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return m[ks[i]] > m[ks[j]] || m[ks[i]] == m[ks[j]] && ks[i] < ks[j] })
	s := ""
	for _, k := range ks {
		s += fmt.Sprintf("%s=%d ", k, m[k])
	}
	return s
}
