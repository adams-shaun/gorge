package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cardc/gorgebe"
	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cardc/lockstep"
	"github.com/adams-shaun/gorge/engine/cardc/mtgish"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// lockstepCmd plays a slice's games on Forge and cardc definitions in both
// directions and prints one TSV line per result, then a totals line.
func lockstepCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lockstep", flag.ContinueOnError)
	fs.SetOutput(stderr)
	slice := fs.String("slice", "fdn", "fdn or legacy")
	from := fs.Int("from", 0, "first FDN pair")
	n := fs.Int("n", 100, "FDN pairs")
	seats := fs.Int("seats", 2, "legacy seat count")
	dirFlag := fs.String("dir", "both", "forward, reverse or both")
	data := fs.String("data", mtgish.DataPath(), "mtgish.lines.json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "cardc: "+format+"\n", a...)
		return 1
	}
	if *slice != "fdn" && *slice != "legacy" {
		return fail("lockstep -slice must be fdn or legacy, not %q", *slice)
	}
	var dirs []lockstep.Direction
	switch *dirFlag {
	case "forward":
		dirs = []lockstep.Direction{lockstep.Forward}
	case "reverse":
		dirs = []lockstep.Direction{lockstep.Reverse}
	case "both":
		dirs = []lockstep.Direction{lockstep.Forward, lockstep.Reverse}
	default:
		return fail("lockstep -dir must be forward, reverse or both, not %q", *dirFlag)
	}
	root, err := mtgish.ModuleRoot()
	if err != nil {
		return fail("%v", err)
	}
	names, err := mtgish.SliceNames(root, *slice)
	if err != nil {
		return fail("%v", err)
	}
	forge, err := cards.OpenCorpus(filepath.Join(root, ".cards"))
	if err != nil {
		return fail("no Forge corpus (run make fetch-cards): %v", err)
	}
	if *data == "" {
		return fail("no mtgish data: run make mtgish-fetch")
	}
	if _, err := os.Stat(*data); err != nil {
		return fail("no mtgish data (run make mtgish-fetch): %v", err)
	}
	all, err := mtgish.LoadAll(*data)
	if err != nil {
		return fail("%v", err)
	}
	ix := mtgish.NewIndex(all)
	var compiled []*ir.Card
	for _, nm := range names {
		c, ok := ix.Lookup(nm)
		if !ok {
			return fail("name %q of slice %s does not resolve", nm, *slice)
		}
		compiled = append(compiled, mtgish.Compile(c))
	}
	fb, err := gorgebe.ReadFallbacks(filepath.Join(root, "engine", "cardc", "testdata", "fallback"))
	if err != nil {
		return fail("%v", err)
	}
	reg, err := gorgebe.Build(forge, compiled, fb)
	if err != nil {
		return fail("%v", err)
	}
	var games []lockstep.Game
	if *slice == "fdn" {
		games, err = lockstep.FDN(root, forge, *from, *n)
	} else {
		var gnames []string
		var decks [][]*cards.Card
		gnames, decks, err = testutil.AcceptanceDecks(forge, *seats)
		if err == nil {
			games = []lockstep.Game{lockstep.Legacy(forge, gnames, decks)}
		}
	}
	if err != nil {
		return fail("%v", err)
	}
	newBot := func(seed uint64) rules.Answerer {
		b := botpolicy.NewGameBot(seed)
		return func(e *rules.Engine, d *decision.Decision) decision.Intent { return b.Answer(e.G, e, d) }
	}
	count := map[string]int{}
	total := 0
	for _, g := range games {
		for _, d := range dirs {
			r := lockstep.Run(g, d, reg.Deck, newBot)
			count[r.Outcome]++
			total++
			detail := strings.NewReplacer("\n", " ", "\t", " ").Replace(r.Detail)
			fmt.Fprintf(stdout, "%s\t%s\t%d\t%s\t%d\t%s\n", r.ID, r.Dir, r.Swapped, r.Outcome, r.Seq, detail)
		}
	}
	fmt.Fprintf(stdout, "total\tgames=%d\tclean=%d\tdiverged=%d\trejected=%d\tunfinished=%d\n",
		total, count["clean"], count["diverged"], count["rejected"], count["unfinished"])
	return 0
}
