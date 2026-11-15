// Package kgen compiles card pools from gorge's compiled card IR (engine/cards)
// into binary tables: each card's static record and its abilities as kop.Instr
// programs, a pool index, and a coverage report naming every IR node it refused
// to lower.
//
//	go build -trimpath -o bin/kgen ./dirac/cmd/kgen && bin/kgen -tables     # every pool's binary table (make dirac-tables)
//	bin/kgen -synth-pools                                             # the synthetic evidence pools' tables (make dirac-tables)
//	go run ./dirac/cmd/kgen -pool x -card-list <file> -cards <dir> -out <dir> # one ad-hoc pool: x.ktab, x.coverage.md, ids.tsv
//	go run ./dirac/cmd/kgen -decks <dir> -cards <dir> -dump <scratch dir>   # IR study dump
//	go run ./dirac/cmd/kgen -census-std -cards <dir> [-sets A,B] [-tsv f] [-all-blockers]   # refusal classes by Standard carry
//	go run ./dirac/cmd/kgen -supported <pool>                               # name<TAB>true|false, every pool entry
//	go run ./dirac/cmd/kgen -show <pool> <card>                             # one card's kop.CardDef literal
//	go run ./dirac/cmd/kgen -ops <pool> -out <file>                         # one pool's OPS.md
//	go run ./dirac/cmd/kgen -refuse <card> -why <text> -repro <repro>       # refuse a card by name (dirac/support/refused)
//	go run ./dirac/cmd/kgen -unrefuse <card>                                # undo -refuse
package kgen

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Main runs kgen with the arguments after the program name and returns its exit status.
func Main(args []string) int {
	flags := flag.NewFlagSet("kgen", flag.ExitOnError)
	poolName := flags.String("pool", "fdn", "pool name")
	decksDir := flags.String("decks", "", "deck pool directory (*.json)")
	cardList := flags.String("card-list", "", "card list file (one name per line) instead of -decks")
	cardsDir := flags.String("cards", ".cards", "compiled card corpus directory")
	outDir := flags.String("out", "", "output directory; with -ops, the output file")
	dumpDir := flags.String("dump", "", "write a per-card IR dump here instead of generating (never commit it)")
	censusOut := flags.Bool("census", false, "print the (context, param) census instead of generating")
	lirOut := flags.Bool("lir", false, "print every card's lowered IR and refusals instead of generating")
	layerCensus := flags.Bool("layer-census", false, "print the interim layer guard's reach (classified cards, refused pairs and deck pairs) instead of generating")
	censusStd := flags.Bool("census-std", false, "print refusal classes ranked by Standard-set carry instead of generating")
	setsFlag := flags.String("sets", "", "with -census-std: comma-separated set codes (default dirac/cardset/standard.txt)")
	allBlockers := flags.Bool("all-blockers", false, "with -census-std: keep lowering past a refusal and report every blocker of a card, not only the first of each node")
	tsvFlag := flags.String("tsv", "", "with -census-std: write a per-refusal TSV dump here")
	whatIf := flags.String("whatif", "", "with -lir: also refuse these cards (comma list) and report coverage")
	tables := flags.Bool("tables", false, "write every pool's binary table under the corpus (make dirac-tables): exit 3 means an ids.tsv changed, rebuild kgen and rerun")
	synthPools := flags.Bool("synth-pools", false, "write the synthetic evidence pools' binary tables under the corpus (make dirac-tables): nothing is written to the source tree")
	supported := flags.String("supported", "", "print name<TAB>true|false for every entry of this pool's table instead of generating")
	unverified := flags.Bool("unverified", false, "with -supported: cards without lockstep evidence count as playable (a refused card stays false)")
	fourSeat := flags.Bool("four-seat", false, "with -supported: a card a pod of three or more seats cannot hold (kop.CardPodLimited: StallTwoSeat at its start) is false")
	show := flags.String("show", "", "print the kop.CardDef literal of the card named by the argument, from this pool's table, instead of generating")
	opsPool := flags.String("ops", "", "write this pool's OPS.md to -out instead of generating")
	refuse := flags.String("refuse", "", "write this card's dirac/support/refused entry instead of generating (with -why and -repro)")
	why := flags.String("why", "", "with -refuse: one line saying what differs")
	class := flags.String("class", "lockstep", "with -refuse: the kind of difference, the refused list's grouping")
	repro := flags.String("repro", "", "with -refuse: the game that shows it, POOL:SURFACE:PAIR[:POLICY[:MULLIGANS]]")
	unrefuse := flags.String("unrefuse", "", "delete this card's dirac/support/refused entry instead of generating")
	flags.Parse(args)
	if *refuse != "" {
		return exitStatus(runRefuse(*cardsDir, *refuse, *class, *why, *repro, os.Stdout))
	}
	if *unrefuse != "" {
		return exitStatus(runUnrefuse(*unrefuse, os.Stdout))
	}
	if *supported != "" {
		return exitStatus(runSupportedView(*supported, *unverified, *fourSeat, os.Stdout))
	}
	if *show != "" {
		return exitStatus(runShow(*show, flags.Arg(0), os.Stdout))
	}
	// From Main, never an init: lowering reads registrations made in other files'
	// inits (exprKinds), and a build from an earlier init silently refuses cards.
	if *tables || *synthPools || *opsPool != "" {
		cards := ""
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "cards" {
				cards = *cardsDir
			}
		})
		if *opsPool != "" {
			return exitStatus(runOps(*opsPool, cards, *outDir))
		}
		if *synthPools {
			return (runSynthPools(cards, os.Stdout, os.Stderr))
		}
		return (runTables(cards, os.Stdout, os.Stderr))
	}
	if *censusStd {
		censusAllBlockers = *allBlockers
		std, err := stdSetCodes(*setsFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, "kgen:", err)
			return 1
		}
		if err := runStdCensus(setlistDir, *cardsDir, std, *tsvFlag, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "kgen:", err)
			return 1
		}
		return 0
	}
	var (
		p   *pool
		err error
	)
	if *cardList != "" {
		p, err = loadCardList(*cardList, *cardsDir)
	} else {
		if *decksDir == "" {
			fmt.Fprintln(os.Stderr, "kgen: pass -card-list or -decks, or -tables for every pool")
			return 2
		}
		p, err = loadPool(*decksDir, *cardsDir)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "kgen:", err)
		return 1
	}
	if *dumpDir != "" {
		if err := dumpPool(p, *dumpDir); err != nil {
			fmt.Fprintln(os.Stderr, "kgen:", err)
			return 1
		}
		return 0
	}
	if *censusOut {
		runCensus(p, os.Stdout)
		return 0
	}
	if *lirOut {
		lw := lowerPool(p)
		for _, c := range append(lw.cards, lw.tokens...) {
			dumpLIR(os.Stdout, c)
		}
		for _, n := range strings.Split(*whatIf, ",") {
			for _, c := range lw.cards {
				if c.Name == strings.TrimSpace(n) {
					c.Refusals = append(c.Refusals, refusal{Card: c.Name, Node: "whatif", Reason: "whatif"})
				}
			}
		}
		if _, _, _, err := buildPool(lw); err != nil {
			fmt.Fprintln(os.Stderr, "kgen:", err)
			return 1
		}
		writeCoverage(os.Stderr, *poolName, computeCoverage(p, lw))
		return 0
	}
	if *layerCensus {
		layerCensusOut = os.Stdout
		if *outDir == "" {
			*outDir = os.TempDir() // ids are read only; nothing is written
		}
	}
	if *outDir == "" {
		fmt.Fprintln(os.Stderr, "kgen: -out is required")
		return 2
	}
	cv, err := generate(p, *poolName, *outDir, filepath.Join(*outDir, idsFile))
	if err != nil {
		fmt.Fprintln(os.Stderr, "kgen:", err)
		return 1
	}
	if *layerCensus {
		return 0
	}
	if cv.decks == 0 {
		fmt.Printf("kgen: %d of %d cards supported\n", cv.cardsOK, cv.cards)
		return 0
	}
	fmt.Printf("kgen: %d of %d decks fully supported (%.2f%%), %d of %d cards\n", cv.supported, cv.decks,
		100*float64(cv.supported)/float64(cv.decks), cv.cardsOK, cv.cards)
	return 0
}

func exitStatus(err error) int {
	if err != nil {
		fmt.Fprintln(os.Stderr, "kgen:", err)
		return 1
	}
	return 0
}
